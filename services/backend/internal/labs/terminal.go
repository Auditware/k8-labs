package labs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// The nested-session terminal (D1). Legacy pod sessions proxy to ttyd running
// in the lab pod (see proxy.go); a nested session has no ttyd - the shell the
// student needs lives in a pod INSIDE the disposable nested cluster. The
// backend already holds that cluster's kubeconfig, so the natural primitive is
// a Kubernetes exec: open an interactive shell in the foothold pod and bridge
// it to the browser over one websocket. Fewer hops and no per-module ttyd image
// than proxying (see docs/architecture/nested-lab-rewrite.md, D1). The frontend
// renders its own xterm.js and speaks this protocol:
//   - browser -> backend: binary frames are stdin; a text frame that parses as
//     {"cols":N,"rows":M} is a PTY resize.
//   - backend -> browser: binary frames are stdout (TTY-merged stderr).

// resizeMsg is the frontend's PTY-resize control message.
type resizeMsg struct {
	Cols uint16 `json:"cols"`
	Rows uint16 `json:"rows"`
}

// nestedTerminal execs a shell into the session's nested foothold pod and
// bridges it to the browser websocket. Called by TerminalProxy for nested
// sessions.
func (m *Manager) nestedTerminal(w http.ResponseWriter, r *http.Request, sess *Session) {
	module, ok := GetModule(sess.ModuleSlug)
	if !ok || module.Nested == nil {
		http.Error(w, "not a nested session", http.StatusBadRequest)
		return
	}

	cfg, err := m.nestedConfigFor(r.Context(), sess.ID)
	if err != nil {
		slog.Error("nested terminal: nested config", "session", sess.ID, "error", err)
		http.Error(w, "terminal unavailable", http.StatusBadGateway)
		return
	}
	executor, err := nestedExecutor(cfg, module.Nested.EntryNamespace, module.Nested.EntryPod)
	if err != nil {
		slog.Error("nested terminal: executor", "session", sess.ID, "error", err)
		http.Error(w, "terminal unavailable", http.StatusBadGateway)
		return
	}

	// Upgrade only after the upstream is ready, so a failure surfaces as a
	// clean HTTP error rather than a websocket that immediately closes.
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("nested terminal: upgrade", "session", sess.ID, "error", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	stdinR, stdinW := io.Pipe()
	sizes := newTermSizeQueue()
	out := &wsWriter{conn: conn}

	// Publish this shell on the session's hub so the AI channel (when granted)
	// can share it: writes go to the same stdin, and stdout is teed to the hub
	// below. Registering stdin here - and clearing it when this terminal closes -
	// is what confines the AI to acting only while the student's terminal is open,
	// with everything it types visible on this very screen. See aihub.go.
	hub := m.hubFor(sess.ID)
	hub.setStdin(stdinW)
	defer hub.clearStdin(stdinW)

	// browser -> stdin / resize. Closing stdin on read error makes the remote
	// shell see EOF and exit, which ends StreamWithContext below.
	go func() {
		defer stdinW.Close()
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if mt == websocket.TextMessage {
				var rs resizeMsg
				if json.Unmarshal(data, &rs) == nil && rs.Cols > 0 && rs.Rows > 0 {
					sizes.push(remotecommand.TerminalSize{Width: rs.Cols, Height: rs.Rows})
					continue
				}
			}
			m.Activity(sess.ID) // real input = the session is in use (idle reaper)
			if _, err := stdinW.Write(data); err != nil {
				return
			}
		}
	}()

	// Keepalive: Cloudflare (and other proxies) cut idle websockets at ~100s,
	// and a shell sitting at a prompt sends nothing. Ping well under that.
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

	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin: stdinR,
		// Tee stdout to the student's websocket AND the hub, so an attached AI
		// reader sees exactly what the student sees (and the hub keeps a
		// scrollback ring for /ai/context). MultiWriter stops on the first write
		// error, which is the browser going away - the same signal that ends the
		// stream anyway.
		Stdout:            io.MultiWriter(out, hubWriter{hub}),
		Tty:               true,
		TerminalSizeQueue: sizes,
	})
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Info("nested terminal stream ended", "session", sess.ID, "error", err)
	}
}

// AITerminalWS attaches the AI channel to a session's shared shell over a
// websocket that speaks the SAME protocol as the browser terminal (binary
// frames = stdin/stdout; text resize frames are ignored - the student's terminal
// owns sizing). It does not start its own exec: it writes into the shell the
// student's terminal already runs, so the AI can only act while that terminal is
// open and everything it types is mirrored on the student's screen. The current
// scrollback is sent first so the AI sees the screen as it stands.
func (m *Manager) AITerminalWS(w http.ResponseWriter, r *http.Request, sessionID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Info("ai terminal: upgrade", "session", sessionID, "error", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	h := m.hubFor(sessionID)
	out := &wsWriter{conn: conn}
	if sb := h.scrollback(); len(sb) > 0 {
		_, _ = out.Write(sb)
	}
	subID, ch := h.subscribe()
	defer h.unsubscribe(subID)

	// AI stdin -> shared shell. A write with no shell attached is dropped (the AI
	// cannot type while the student's terminal is closed); the connection stays up
	// so it resumes the moment the terminal reconnects.
	go func() {
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				cancel()
				return
			}
			if mt == websocket.TextMessage {
				continue // AI does not drive PTY resize; the browser owns it
			}
			_ = h.writeStdin(data)
			m.Activity(sessionID)
		}
	}()

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

	for {
		select {
		case <-ctx.Done():
			return
		case p, ok := <-ch:
			if !ok {
				return // hub dropped (session torn down)
			}
			if _, err := out.Write(p); err != nil {
				return
			}
		}
	}
}

// shellCommand is what the exec runs. A bare /bin/sh on an Alpine/BusyBox image
// renders BusyBox ash's default prompt - just the working directory and $/#
// ("/ #") - which drops the user@host that every other terminal shows. This
// sets a bash-style PS1 so the prompt names who you are, which matters most in
// a privesc lab: the prompt flips from you@host to root@host the instant an
// escalation lands, instead of a subtle $ -> # you have to notice.
//
// It stays within the "lowest common denominator" the bare /bin/sh had, by
// leaning only on POSIX guarantees:
//   - `export PS1=...; exec /bin/sh` is plain POSIX. `export` and a
//     single-quoted assignment work in every real /bin/sh, so this can't fail
//     any shell that a bare `/bin/sh` would have run - if /bin/sh is missing
//     the exec fails identically either way. No new failure mode.
//   - `export` (not a bare assignment) so the value is inherited by any shell a
//     lab spawns - notably the root shell backup-tool execs after setuid(0),
//     which should read root@host too.
//   - `exec` replaces the wrapper, so exactly one shell process stays attached
//     to the PTY (no orphaned parent, no doubled Ctrl-C target).
//
// The \u \h \w \$ escapes are interpreted by the shell at prompt-render time,
// not by us. BusyBox ash and bash (the only shells the foothold images ship)
// understand them; a shell that didn't would print them literally - a cosmetic
// wart, never a broken session.
//
// The color is ANSI SGR wrapped in \[ \] (BusyBox ash strips those non-printing
// markers, so the cursor math stays right on long lines - verified on the
// foothold image). Semantic slots, not raw RGB: the actual hues live in the
// xterm theme (NestedTerminal.tsx), tuned for the navy background there.
//   - \u@\h in cyan (\033[36m): who and where. \u re-resolves per prompt, so it
//     already reads root@host the moment you escalate.
//   - \w in bright-black/gray (\033[90m): the path, kept quiet.
//   - \$ in green (\033[32m): the sigil, a color of its own so it stands apart
//     from the name. \$ renders `$` for a normal user and `#` for root, so the
//     glyph itself carries the privilege change.
//
// One honest limit: the sigil's COLOR can't switch on escalation. ash's PS1 is
// a static string with no per-prompt uid branch (no PROMPT_COMMAND, no command
// substitution), and the root shell backup-tool spawns just inherits this exact
// string - so root sees a green `#`, not a red one. The privilege cue is
// carried by the glyph ($ -> #) and the name (user -> root), not the sigil
// color. Kept to the 8 basic SGR codes so any ANSI-aware terminal renders it.
const shellCommand = `export PS1='\[\033[36m\]\u@\h\[\033[0m\]:\[\033[90m\]\w\[\033[0m\] \[\033[32m\]\$\[\033[0m\] '; exec /bin/sh`

// nestedExecutor builds a TTY exec executor for a shell in the given pod on the
// nested cluster. /bin/sh is the lowest common denominator (alpine-based
// foothold images have it; so does anything bash-capable).
func nestedExecutor(cfg *rest.Config, namespace, pod string) (remotecommand.Executor, error) {
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	req := cs.CoreV1().RESTClient().Post().
		Resource("pods").Name(pod).Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: []string{"/bin/sh", "-c", shellCommand},
			Stdin:   true,
			Stdout:  true,
			Stderr:  false, // TTY merges stderr into stdout
			TTY:     true,
		}, scheme.ParameterCodec)
	return remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
}

// nestedConfigFor returns a REST config for the session's nested cluster,
// using the cached kubeconfig when present and re-fetching it from the runner
// pod otherwise (e.g. a session reconciled after a backend restart, whose cache
// was lost). Repopulates the cache on a re-fetch.
func (m *Manager) nestedConfigFor(ctx context.Context, id string) (*rest.Config, error) {
	m.mu.Lock()
	sess, ok := m.sessions[id]
	var kc []byte
	if ok {
		kc = sess.nestedKubeconfig
	}
	m.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("session %s not found", id)
	}
	if len(kc) == 0 {
		fetched, err := m.fetchNestedKubeconfig(ctx, id)
		if err != nil {
			return nil, err
		}
		kc = fetched
		m.mu.Lock()
		if s, ok := m.sessions[id]; ok {
			s.nestedKubeconfig = kc
		}
		m.mu.Unlock()
	}
	return nestedRESTConfig(kc)
}

// wsWriter adapts a websocket connection to io.Writer for exec stdout, with a
// mutex so the stdout stream and the keepalive-ping goroutine don't write the
// connection concurrently (gorilla permits only one writer at a time).
type wsWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (w *wsWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.conn.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *wsWriter) ping() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
}

// termSizeQueue feeds PTY resize events to remotecommand. Non-blocking push
// (drop if full) - a missed intermediate resize is harmless, the next one wins.
type termSizeQueue struct {
	ch chan remotecommand.TerminalSize
}

func newTermSizeQueue() *termSizeQueue {
	return &termSizeQueue{ch: make(chan remotecommand.TerminalSize, 4)}
}

func (q *termSizeQueue) Next() *remotecommand.TerminalSize {
	s, ok := <-q.ch
	if !ok {
		return nil
	}
	return &s
}

func (q *termSizeQueue) push(s remotecommand.TerminalSize) {
	select {
	case q.ch <- s:
	default:
	}
}
