package labs

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// waitForDefenderPod must resolve the sensor DaemonSet's generated pod name by
// label and skip a non-running pod - the log stream can only attach to a running
// one. Getting the selector wrong would leave the pane hanging on "not ready".
func TestWaitForDefenderPod(t *testing.T) {
	spec := &DefenderSpec{Namespace: "tetragon", PodLabel: "app=tetragon", Container: "export-stdout"}
	cs := fake.NewSimpleClientset(
		// A pending pod with the right label must NOT be picked.
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "tetragon-pending", Namespace: "tetragon", Labels: map[string]string{"app": "tetragon"}},
			Status:     corev1.PodStatus{Phase: corev1.PodPending},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "tetragon-abc", Namespace: "tetragon", Labels: map[string]string{"app": "tetragon"}},
			Status:     corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)
	name, err := waitForDefenderPod(context.Background(), cs, spec, time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if name != "tetragon-abc" {
		t.Errorf("resolved pod = %q, want the running tetragon-abc", name)
	}
}

func TestWaitForDefenderPod_NoneRunning(t *testing.T) {
	spec := &DefenderSpec{Namespace: "tetragon", PodLabel: "app=tetragon", Container: "export-stdout"}
	cs := fake.NewSimpleClientset() // nothing matches
	if _, err := waitForDefenderPod(context.Background(), cs, spec, 50*time.Millisecond); err == nil {
		t.Error("expected an error when no running sensor pod exists")
	}
}

// The Defender pane resolves the sensor pod by PodLabel; a malformed selector
// would make GetLogs match nothing and the pane hang on "sensor not ready", so
// assert the eBPF module ships a well-formed one and names a container.
func TestDefenderSpec_EBPFModuleWellFormed(t *testing.T) {
	m, ok := GetModule("ebpf-detect")
	if !ok {
		t.Fatal("ebpf-detect not registered")
	}
	if !m.EBPF {
		t.Error("ebpf-detect must be EBPF=true")
	}
	if m.Nested == nil || m.Nested.Defender == nil {
		t.Fatal("ebpf-detect must carry a Defender spec")
	}
	d := m.Nested.Defender
	if _, ok := defenderLabelSelector(d); !ok {
		t.Errorf("Defender.PodLabel %q is not a well-formed key=value selector", d.PodLabel)
	}
	if d.Namespace == "" || d.Container == "" {
		t.Errorf("Defender spec needs Namespace and Container, got %+v", d)
	}
}

// Only eBPF modules have a sensor to watch; the others must not advertise a
// Defender feed (the route 400s on a nil Defender, and the frontend hides the
// tab for non-eBPF sessions).
func TestDefenderSpec_NonEBPFModulesHaveNone(t *testing.T) {
	for _, m := range ListModules() {
		if m.usesEBPFKernel() {
			continue
		}
		if m.Nested != nil && m.Nested.Defender != nil {
			t.Errorf("%s is not eBPF but carries a Defender spec", m.Slug)
		}
	}
}

// HasDefender is what the frontend gates the Defender tab on, so it must equal
// "this module actually ships a Defender pane" for every module - no more, no
// less. The research labs are the reason this is a separate flag from EBPF: they
// have a Defender pane but select their kernel via Kernel, not EBPF, so gating
// the tab on EBPF (as it once was) hid their sensor. This locks the invariant
// init() establishes and guards against that regression returning.
func TestHasDefenderMatchesDefenderSpec(t *testing.T) {
	sawResearchWithDefender := false
	for _, m := range ListModules() {
		want := m.Nested != nil && m.Nested.Defender != nil
		if m.HasDefender != want {
			t.Errorf("%s: HasDefender=%v but has-a-Defender-spec=%v (init() must keep them in step)", m.Slug, m.HasDefender, want)
		}
		if m.IsResearch() {
			if !m.HasDefender {
				t.Errorf("%s: research lab must advertise a Defender pane (its description promises one)", m.Slug)
			}
			if m.EBPF {
				t.Errorf("%s: research labs set Kernel, not EBPF - if EBPF were true the tab-gating bug would be masked", m.Slug)
			}
			sawResearchWithDefender = sawResearchWithDefender || m.HasDefender
		}
	}
	if !sawResearchWithDefender {
		t.Error("expected at least one research lab with a Defender pane - the case this flag exists for")
	}
}

// The Defender flag must reach the browser as the "defender" JSON key (the
// frontend's Module.defender), and must be omitted, not false, for labs without
// a pane - the tab-gating reads a missing key as no-tab.
func TestHasDefenderJSONTag(t *testing.T) {
	research, _ := GetModule("driver-research-dev")
	if b, _ := json.Marshal(research); !strings.Contains(string(b), `"defender":true`) {
		t.Errorf("driver-research-dev JSON must carry \"defender\":true, got %s", b)
	}
	plain, _ := GetModule("recon-first-contact")
	if b, _ := json.Marshal(plain); strings.Contains(string(b), `"defender"`) {
		t.Errorf("a lab without a Defender pane must omit the defender key, got %s", b)
	}
}

// defenderEventSummary is the load-bearing parse for the AI's /context eBPF
// feed: it turns Tetragon's loosely-typed JSON into a compact one-liner and
// drops everything else. Mirrors the frontend DefenderPane.parse shapes.
func TestDefenderEventSummary(t *testing.T) {
	cases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "process_exec with args",
			line: `{"process_exec":{"process":{"binary":"/usr/bin/id","arguments":"-u"}}}`,
			want: "exec /usr/bin/id -u",
		},
		{
			name: "process_exec no args",
			line: `{"process_exec":{"process":{"binary":"/bin/ls"}}}`,
			want: "exec /bin/ls",
		},
		{
			name: "commit_creds with cred uid/euid",
			line: `{"process_kprobe":{"function_name":"commit_creds","process":{"binary":"/app/fuzz/fuzz"},"args":[{"process_credentials_arg":{"uid":0,"euid":0,"suid":0}}]}}`,
			want: "commit_creds uid=0 euid=0 via /app/fuzz/fuzz",
		},
		{
			name: "commit_creds escalation (euid 0, uid not 0)",
			line: `{"process_kprobe":{"function_name":"commit_creds","process":{"binary":"/usr/bin/backup-tool"},"args":[{"process_credentials_arg":{"uid":10001,"euid":0}}]}}`,
			want: "commit_creds uid=10001 euid=0 via /usr/bin/backup-tool",
		},
		{
			name: "commit_creds no binary falls back",
			line: `{"process_kprobe":{"function_name":"commit_creds","args":[{"process_credentials_arg":{"uid":100,"euid":100}}]}}`,
			want: "commit_creds uid=100 euid=100 via a process",
		},
		{name: "other kprobe dropped", line: `{"process_kprobe":{"function_name":"tcp_connect"}}`, want: ""},
		{name: "unrelated event dropped", line: `{"process_exit":{"process":{"binary":"/bin/sh"}}}`, want: ""},
		{name: "exec with empty binary dropped", line: `{"process_exec":{"process":{"binary":""}}}`, want: ""},
		{name: "garbage line dropped", line: `not json`, want: ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := defenderEventSummary([]byte(c.line)); got != c.want {
				t.Errorf("defenderEventSummary(%s) = %q, want %q", c.line, got, c.want)
			}
		})
	}
}
