package labs

// Cross-package test support. These are exported so tests in other packages
// (notably internal/api's /ai grant + terminal wiring) can build a Manager and
// seed a live session without a Kubernetes cluster. They touch only the
// in-memory maps and are never used on a production path.

// NewManagerForTest returns a Manager with session/AI state initialized and no
// Kubernetes client.
func NewManagerForTest() *Manager {
	return &Manager{
		sessions: map[string]*Session{},
		ended:    map[string]EndedSession{},
		fetching: map[string]bool{},
		ai:       newAIState(),
	}
}

// SetSessionForTest inserts a live session directly, bypassing provisioning.
func (m *Manager) SetSessionForTest(s *Session) {
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
}

// AttachEchoShellForTest simulates a student terminal being open: it registers a
// stdin writer on the session's hub that echoes whatever is written back out as
// terminal output. Lets a test exercise the full AIExec/mirror loop (write a
// command, read it back) without a real nested cluster.
func (m *Manager) AttachEchoShellForTest(sessionID string) {
	h := m.hubFor(sessionID)
	h.setStdin(hubWriter{h})
}
