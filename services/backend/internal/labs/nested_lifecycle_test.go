package labs

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// These exercise the orchestration branches a refactor is most likely to break:
// that Delete tears down the runner, that Reconcile re-adopts orphaned runner
// pods, and that Create registers a session and returns immediately. They run
// against a fake clientset - fast, CI-friendly, no real cluster. The slow
// provisioning internals (exec, dialing the nested API) aren't fakeable and are
// covered by the manual k3d/AX41 e2e instead (see docs/architecture/
// nested-lab-rewrite.md).

func lifecycleCfg() Config {
	return Config{RunnerNamespace: "labs-runners", RunnerImage: "reg/session-runner:dev"}
}

func runnerPodObj(id, owner, module string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: runnerName(id), Namespace: "labs-runners", Labels: runnerLabels(id, owner, Module{Slug: module}),
	}}
}

// Reconcile must re-adopt orphaned runner pods after a restart, recovering the
// owner and module from the pod's labels - otherwise the session is unreapable
// and its owner can't stop it (see Reconcile's doc).
func TestReconcile_AdoptsRunners(t *testing.T) {
	cs := fake.NewSimpleClientset(runnerPodObj("r1", "u1", "ns-escape"))
	m := NewManager(cs, nil, lifecycleCfg())
	if err := m.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	r1, ok := m.Get("r1")
	if !ok {
		t.Fatal("runner r1 not adopted")
	}
	if r1.Namespace != "labs-runners" || r1.OwnerID != "u1" || r1.ModuleSlug != "ns-escape" {
		t.Errorf("r1 reconciled wrong: %+v", r1)
	}
}

// Reconcile must ignore pods missing the session/module labels rather than
// adopting a half-identified session it could never reap correctly.
func TestReconcile_SkipsUnlabelledPods(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "runner-x", Namespace: "labs-runners", Labels: map[string]string{"app": "session-runner"},
	}})
	m := NewManager(cs, nil, lifecycleCfg())
	if err := m.Reconcile(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := len(m.List()); got != 0 {
		t.Errorf("adopted %d sessions from an unlabelled pod, want 0", got)
	}
}

// Delete removes the one runner pod (which collapses the whole nested cluster)
// and forgets the session.
func TestDelete_DeletesRunner(t *testing.T) {
	cs := fake.NewSimpleClientset(runnerPodObj("r1", "u1", "ns-escape"))
	m := NewManager(cs, nil, lifecycleCfg())
	m.sessions["r1"] = &Session{ID: "r1", ModuleSlug: "ns-escape", Namespace: "labs-runners", Status: "running"}

	if err := m.Delete(context.Background(), "r1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := cs.CoreV1().Pods("labs-runners").Get(context.Background(), runnerName("r1"), metav1.GetOptions{}); !apierrors.IsNotFound(err) {
		t.Errorf("runner pod should be gone, got err=%v", err)
	}
	if _, ok := m.Get("r1"); ok {
		t.Error("session still tracked after Delete")
	}
}

// Create registers the session and returns immediately with status "starting" -
// provisioning happens in the background (asserting only the synchronous,
// race-free routing here; the async build is covered by the manual e2e).
func TestCreate_RegistersSession(t *testing.T) {
	m := NewManager(fake.NewSimpleClientset(), nil, lifecycleCfg())
	sess, err := m.Create(context.Background(), "ns-escape", "u1", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if sess.Status != "starting" {
		t.Errorf("status = %q, want starting", sess.Status)
	}
	if sess.OwnerID != "u1" || sess.Namespace != "labs-runners" {
		t.Errorf("session fields wrong: %+v", sess)
	}
	if _, ok := m.Get(sess.ID); !ok {
		t.Error("session not registered")
	}
}

// A module with no Nested spec is a registry bug, not a second supported shape -
// Create must reject it rather than silently provisioning nothing. This is the
// guard that replaces the old legacy-vs-nested branch (see F1).
func TestCreate_RejectsModuleWithoutNestedSpec(t *testing.T) {
	m := NewManager(fake.NewSimpleClientset(), nil, lifecycleCfg())
	modules["broken-test-module"] = Module{Slug: "broken-test-module"}
	defer delete(modules, "broken-test-module")

	if _, err := m.Create(context.Background(), "broken-test-module", "u1", nil); err == nil {
		t.Fatal("Create accepted a module with no nested spec, want error")
	}
}
