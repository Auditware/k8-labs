package labs

import (
	"testing"
	"time"
)

func newTestManager() *Manager {
	return &Manager{
		sessions: map[string]*Session{},
		ended:    map[string]EndedSession{},
		fetching: map[string]bool{},
		ai:       newAIState(),
	}
}

func TestAIGrantLifecycle(t *testing.T) {
	m := newTestManager()

	if m.VerifyAIGrant("s1", "jti-1") {
		t.Fatal("no grant issued yet, VerifyAIGrant should be false")
	}

	m.GrantAI("s1", "jti-1", time.Hour)
	if !m.VerifyAIGrant("s1", "jti-1") {
		t.Fatal("fresh grant should verify")
	}
	if m.VerifyAIGrant("s1", "wrong-jti") {
		t.Fatal("a different jti must not verify")
	}
	if m.VerifyAIGrant("other", "jti-1") {
		t.Fatal("grant is per-session; another session must not verify")
	}

	// Re-issuing replaces the grant: the old jti is instantly dead.
	m.GrantAI("s1", "jti-2", time.Hour)
	if m.VerifyAIGrant("s1", "jti-1") {
		t.Fatal("re-issuing must invalidate the previous jti")
	}
	if !m.VerifyAIGrant("s1", "jti-2") {
		t.Fatal("newest jti should verify")
	}

	// Explicit revoke (panel close).
	m.RevokeAI("s1")
	if m.VerifyAIGrant("s1", "jti-2") {
		t.Fatal("revoked grant must not verify")
	}
}

func TestAIGrantExpires(t *testing.T) {
	m := newTestManager()
	m.GrantAI("s1", "jti-1", -time.Second) // already expired
	if m.VerifyAIGrant("s1", "jti-1") {
		t.Fatal("expired grant must not verify")
	}
}

func TestHubStdinRequiresShell(t *testing.T) {
	h := newTermHub()
	if err := h.writeStdin([]byte("ls\n")); err != ErrNoShell {
		t.Fatalf("writeStdin with no shell = %v, want ErrNoShell", err)
	}

	var got []byte
	h.setStdin(writerFunc(func(p []byte) (int, error) { got = append(got, p...); return len(p), nil }))
	if err := h.writeStdin([]byte("id\n")); err != nil {
		t.Fatalf("writeStdin with shell attached: %v", err)
	}
	if string(got) != "id\n" {
		t.Fatalf("shell received %q, want %q", got, "id\n")
	}
}

func TestHubBroadcastAndScrollback(t *testing.T) {
	h := newTermHub()
	subID, ch := h.subscribe()
	defer h.unsubscribe(subID)

	h.broadcast([]byte("hello "))
	h.broadcast([]byte("world"))

	if got := string(h.scrollback()); got != "hello world" {
		t.Fatalf("scrollback = %q, want %q", got, "hello world")
	}
	// Subscriber sees each frame.
	if got := string(<-ch); got != "hello " {
		t.Fatalf("first frame = %q", got)
	}
	if got := string(<-ch); got != "world" {
		t.Fatalf("second frame = %q", got)
	}
}

func TestExecDone(t *testing.T) {
	cmd := "kubectl auth can-i --list"
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"echo only, no output yet", "root@h:/ # " + cmd + "\n", false},
		{"output streaming, no prompt", "root@h:/ # " + cmd + "\nsecrets  [get li", false},
		{"prompt returned after output", "root@h:/ # " + cmd + "\nsecrets  [get list]\nroot@h:/ # ", true},
		{"with ansi codes", "\x1b[36mroot@h\x1b[0m:/ # " + cmd + "\r\nsecrets\r\n\x1b[36mroot@h\x1b[0m:/ # ", true},
		{"command not found (wrapped echo)", "root@h:/ # kube\nctl auth\nsecrets\nroot@h:/ # ", false},
	}
	for _, c := range cases {
		if got := execDone([]byte(c.raw), cmd); got != c.want {
			t.Errorf("%s: execDone = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestAIExecNoShell(t *testing.T) {
	m := newTestManager()
	if _, err := m.AIExec("s1", "id"); err != ErrNoShell {
		t.Fatalf("AIExec with no terminal = %v, want ErrNoShell", err)
	}
}

// writerFunc adapts a func to io.Writer for the stdin test.
type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) { return f(p) }

// A long-running job produces output but no input. Before shell output counted
// as activity, AIExec marked the session once at launch and a 90-minute build
// idled the session out from under itself; these cover the output path.
func TestBroadcastMarksActivity(t *testing.T) {
	m := newTestManager()
	stale := time.Now().Add(-30 * time.Minute)
	m.SetSessionForTest(&Session{
		ID:           "s1",
		Status:       "running",
		CreatedAt:    stale,
		LastActivity: stale,
		Timing:       Timing{MaxAge: 4 * time.Hour, Idle: 60 * time.Minute},
	})

	m.hubFor("s1").broadcast([]byte("[ 1234.5 ] fuzzing...\n"))

	m.mu.Lock()
	got := m.sessions["s1"].LastActivity
	m.mu.Unlock()
	if !got.After(stale) {
		t.Fatalf("shell output must reset the idle clock: LastActivity %v, want after %v", got, stale)
	}
}

func TestBroadcastActivityIsThrottled(t *testing.T) {
	m := newTestManager()
	m.SetSessionForTest(&Session{
		ID:     "s1",
		Status: "running",
		Timing: Timing{MaxAge: 4 * time.Hour, Idle: 60 * time.Minute},
	})
	h := m.hubFor("s1")

	h.broadcast([]byte("first"))
	m.mu.Lock()
	first := m.sessions["s1"].LastActivity
	m.mu.Unlock()

	// A chatty job floods the hub; the throttle must keep the manager lock out of
	// the per-chunk path, so the second burst does not re-mark within the window.
	h.broadcast([]byte("second"))
	m.mu.Lock()
	second := m.sessions["s1"].LastActivity
	m.mu.Unlock()
	if !second.Equal(first) {
		t.Fatalf("activity re-marked inside the throttle window: %v then %v", first, second)
	}

	// Once the window lapses, output marks activity again.
	h.mu.Lock()
	h.lastOutputPing = time.Now().Add(-outputActivityThrottle - time.Second)
	h.mu.Unlock()
	h.broadcast([]byte("third"))
	m.mu.Lock()
	third := m.sessions["s1"].LastActivity
	m.mu.Unlock()
	if !third.After(second) {
		t.Fatalf("output after the throttle window must re-mark activity: %v not after %v", third, second)
	}
}
