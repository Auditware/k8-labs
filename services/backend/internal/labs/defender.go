package labs

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// The Defender pane (Slice C). eBPF modules (Module.EBPF) run an in-guest
// Tetragon sensor INSIDE the nested cluster; this streams that sensor's JSON
// event feed to the browser so a student can watch their own lab being observed
// - the eBPF is the product, not admin surveillance (see
// docs/architecture/ebpf-per-session.md).
//
// It is the terminal bridge's read-only cousin (terminal.go): the backend
// already holds the nested kubeconfig, so the natural primitive is following the
// sensor pod's container log via the nested API and pumping each line to one
// websocket. Deliberately NOT the Alloy->Loki pipeline: the sensor's output is a
// NESTED pod log that never reaches outer Alloy/Loki (nested activity is
// attributed to namespace "" and dropped by the export-denylist - see
// apps/tetragon/templates/process-creds-changed.yaml). One text frame per event
// line; no stdin (the pane is read-only).

// defenderBacklog is how many recent event lines a freshly-opened pane receives
// before live-following, so the feed isn't blank until the next kernel event.
const defenderBacklog = 50

// DefenderStream follows the session's in-guest sensor and bridges its event
// feed to the browser websocket. Called by the API layer after auth+ownership.
func (m *Manager) DefenderStream(w http.ResponseWriter, r *http.Request, sess *Session) {
	module, ok := GetModule(sess.ModuleSlug)
	if !ok || module.Nested == nil || module.Nested.Defender == nil {
		http.Error(w, "no defender feed for this session", http.StatusBadRequest)
		return
	}
	spec := module.Nested.Defender

	cfg, err := m.nestedConfigFor(r.Context(), sess.ID)
	if err != nil {
		slog.Error("defender: nested config", "session", sess.ID, "error", err)
		http.Error(w, "defender unavailable", http.StatusBadGateway)
		return
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		slog.Error("defender: clientset", "session", sess.ID, "error", err)
		http.Error(w, "defender unavailable", http.StatusBadGateway)
		return
	}

	// The sensor is a DaemonSet, so its pod name is generated - resolve it by
	// label. A just-provisioned session may still be starting the sensor, so give
	// it a short grace window rather than failing the connect outright.
	pod, err := waitForDefenderPod(r.Context(), cs, spec, 20*time.Second)
	if err != nil {
		slog.Error("defender: sensor pod", "session", sess.ID, "error", err)
		http.Error(w, "sensor not ready", http.StatusBadGateway)
		return
	}

	// Open the log stream BEFORE upgrading, so a failure is a clean HTTP error
	// rather than a websocket that instantly closes (same order as the terminal).
	tail := int64(defenderBacklog)
	logs, err := cs.CoreV1().Pods(spec.Namespace).GetLogs(pod, &corev1.PodLogOptions{
		Container: spec.Container,
		Follow:    true,
		TailLines: &tail,
	}).Stream(r.Context())
	if err != nil {
		slog.Error("defender: log stream", "session", sess.ID, "error", err)
		http.Error(w, "defender unavailable", http.StatusBadGateway)
		return
	}
	defer logs.Close()

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("defender: upgrade", "session", sess.ID, "error", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	out := &wsWriter{conn: conn}

	// Drain the browser->server direction: the pane sends nothing, but reading is
	// what surfaces the close (and keeps control frames like pong flowing). Any
	// read error means the tab is gone -> tear the stream down.
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	// Keepalive: same ~100s idle-proxy cut the terminal guards against, and an
	// idle sensor (no new kernel events) sends nothing.
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				if err := out.ping(); err != nil {
					cancel()
					return
				}
			}
		}
	}()

	// Pump event lines. Tetragon writes one JSON object per line; forward each as
	// a text frame and let the frontend parse. A very long line (unbounded kernel
	// arg strings) shouldn't wedge the scanner, so grow the buffer generously.
	sc := bufio.NewScanner(logs)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if ctx.Err() != nil {
			return
		}
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := out.writeText(line); err != nil {
			return
		}
	}
	if err := sc.Err(); err != nil && ctx.Err() == nil && !errors.Is(err, context.Canceled) {
		slog.Info("defender stream ended", "session", sess.ID, "error", err)
	}
}

// defenderEventSummary condenses one Tetragon JSON event line into a short,
// token-light string for the AI's /context. It mirrors the frontend
// DefenderPane's parse (process_exec + the commit_creds kprobe with its cred
// uid/euid) but stays deliberately minimal - the model reasons over the rest.
// Returns "" for a line that carries nothing worth surfacing, so callers drop it.
func defenderEventSummary(line []byte) string {
	var e struct {
		ProcessExec *struct {
			Process struct {
				Binary    string `json:"binary"`
				Arguments string `json:"arguments"`
			} `json:"process"`
		} `json:"process_exec"`
		ProcessKprobe *struct {
			FunctionName string `json:"function_name"`
			Process      struct {
				Binary string `json:"binary"`
			} `json:"process"`
			Args []struct {
				Cred *struct {
					UID  *int `json:"uid"`
					EUID *int `json:"euid"`
				} `json:"process_credentials_arg"`
			} `json:"args"`
		} `json:"process_kprobe"`
	}
	if json.Unmarshal(line, &e) != nil {
		return ""
	}
	if e.ProcessExec != nil && e.ProcessExec.Process.Binary != "" {
		s := "exec " + e.ProcessExec.Process.Binary
		if e.ProcessExec.Process.Arguments != "" {
			s += " " + e.ProcessExec.Process.Arguments
		}
		return s
	}
	if pk := e.ProcessKprobe; pk != nil && pk.FunctionName == "commit_creds" {
		bin := pk.Process.Binary
		if bin == "" {
			bin = "a process"
		}
		n := func(v *int) string {
			if v == nil {
				return "?"
			}
			return strconv.Itoa(*v)
		}
		for _, a := range pk.Args {
			if a.Cred != nil {
				return "commit_creds uid=" + n(a.Cred.UID) + " euid=" + n(a.Cred.EUID) + " via " + bin
			}
		}
		return "commit_creds via " + bin
	}
	return ""
}

// DefenderRecent returns up to n compact summaries of the most recent in-guest
// sensor events for a session that ships one (Module.Nested.Defender), for the
// AI's /context. It is the read-only, snapshot cousin of DefenderStream: it does
// NOT follow - it pulls the tail of the sensor log and closes - so it is safe to
// call on each /context poll. Best-effort by contract: (nil, nil) for a module
// with no sensor, and a real error only on a genuine fetch failure the caller may
// log and ignore.
func (m *Manager) DefenderRecent(ctx context.Context, sess *Session, n int) ([]string, error) {
	module, ok := GetModule(sess.ModuleSlug)
	if !ok || module.Nested == nil || module.Nested.Defender == nil {
		return nil, nil
	}
	spec := module.Nested.Defender

	cfg, err := m.nestedConfigFor(ctx, sess.ID)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	// Short grace only: /context must stay responsive, so if the sensor pod isn't
	// up yet we return nothing rather than blocking the poll on the 20s wait the
	// live stream can afford.
	pod, err := waitForDefenderPod(ctx, cs, spec, 2*time.Second)
	if err != nil {
		return nil, err
	}

	// Over-read: many raw lines summarise to "" (dropped), so pull a wider tail
	// than n and trim after filtering.
	tail := int64(n * 6)
	logs, err := cs.CoreV1().Pods(spec.Namespace).GetLogs(pod, &corev1.PodLogOptions{
		Container: spec.Container,
		TailLines: &tail,
	}).Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer logs.Close()

	var out []string
	sc := bufio.NewScanner(logs)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if s := defenderEventSummary(sc.Bytes()); s != "" {
			out = append(out, s)
		}
	}
	if len(out) > n {
		out = out[len(out)-n:]
	}
	return out, nil
}

// waitForDefenderPod returns the name of a running sensor pod matching the
// spec's label, polling until one is Running or the deadline passes.
func waitForDefenderPod(ctx context.Context, cs kubernetes.Interface, spec *DefenderSpec, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for {
		pods, err := cs.CoreV1().Pods(spec.Namespace).List(ctx, metav1.ListOptions{LabelSelector: spec.PodLabel})
		if err == nil {
			for i := range pods.Items {
				if pods.Items[i].Status.Phase == corev1.PodRunning {
					return pods.Items[i].Name, nil
				}
			}
		}
		if time.Now().After(deadline) {
			if err != nil {
				return "", err
			}
			return "", fmt.Errorf("no running pod for %q in %s", spec.PodLabel, spec.Namespace)
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// writeText sends one event line as a websocket text frame, sharing wsWriter's
// mutex with the keepalive ping (gorilla allows only one concurrent writer).
func (w *wsWriter) writeText(p []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteMessage(websocket.TextMessage, p)
}

// defenderLabelSelector is a tiny guard used by tests to assert the module's
// PodLabel is a well-formed "key=value" selector (a malformed one would make
// GetLogs list nothing and the pane hang on "sensor not ready").
func defenderLabelSelector(spec *DefenderSpec) (string, bool) {
	k, v, ok := strings.Cut(spec.PodLabel, "=")
	return spec.PodLabel, ok && k != "" && v != ""
}
