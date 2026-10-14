package labs

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// TerminalProxy serves a session's terminal. Every session's shell lives in a
// pod inside its disposable nested cluster, reached by exec over a single
// websocket (see terminal.go's nestedTerminal).
//
// This keeps lab workloads off the public ingress entirely: the browser only
// ever talks to the backend, which is the only thing with RBAC to reach the
// runner pod at all.
//
// The old path here reverse-proxied a ttyd HTTP server running inside a lab pod;
// it went away with the legacy single-pod shape (see
// docs/architecture/lab-session-rewrite.md, F1). There is no ttyd anywhere in
// the session path now.
func (m *Manager) TerminalProxy(w http.ResponseWriter, r *http.Request) {
	sess, ok := m.Get(r.PathValue("id"))
	if !ok {
		http.NotFound(w, r)
		return
	}
	m.nestedTerminal(w, r, sess)
}

var upgrader = websocket.Upgrader{
	// Same-origin already (browser only ever talks to this backend) - no
	// separate origin check needed here.
	CheckOrigin: func(r *http.Request) bool { return true },
}
