package labs

import (
	"errors"
	"io"
	"regexp"
	"strings"
	"sync"
	"time"
)

// AI lab-assist plumbing. Two pieces:
//
//   - termHub lets the AI channel share the SAME nested shell the student is
//     watching. The student's terminal owns the shell (it sets stdin on connect,
//     clears it on disconnect); the AI can only write while that terminal is
//     open, so every AI keystroke lands on the student's screen by construction -
//     there is no hidden second shell.
//   - aiGrant is the short-lived, server-revocable capability the /ai/* routes
//     check. The live grant (not the JWT exp alone) is authoritative, so a lab
//     closed or a panel closed kills access immediately, before the 15m expiry.
//
// See internal/api/ai.go for the routes and docs/architecture (pending).

// scrollbackCap bounds the recent-output ring the AI reads as context. A few
// screens of an 80-col terminal; enough to see the last command's result without
// letting the buffer grow without bound.
const scrollbackCap = 16 * 1024

var (
	// ErrNoShell is returned when the AI tries to type but no student terminal is
	// attached - which is how the AI is prevented from acting invisibly.
	ErrNoShell = errors.New("no active terminal for this session")
)

// outputActivityThrottle bounds how often shell output is allowed to bump the
// idle clock. Output arrives in tiny chunks - a chatty fuzzer produces thousands
// per second - and each bump takes the manager-wide lock, so firing per chunk
// would be pure contention. Once per interval is plenty: the shortest idle
// window is measured in minutes.
const outputActivityThrottle = 30 * time.Second

// termHub fans one nested shell out to many readers and their stdin back in.
type termHub struct {
	mu      sync.Mutex
	stdin   io.Writer           // live shell's stdin; nil when no terminal attached
	subs    map[int]chan []byte // output subscribers (AI readers)
	nextSub int
	ring    []byte // recent output, capped at scrollbackCap
	// onOutput marks session activity when the shell produces output, throttled
	// to outputActivityThrottle by lastOutputPing. Set by hubFor; nil in tests
	// that build a bare hub. See broadcast for why output counts as activity.
	onOutput       func()
	lastOutputPing time.Time
}

func newTermHub() *termHub {
	return &termHub{subs: map[int]chan []byte{}}
}

// setStdin registers the active shell's stdin (browser terminal on connect).
func (h *termHub) setStdin(w io.Writer) {
	h.mu.Lock()
	h.stdin = w
	h.mu.Unlock()
}

// clearStdin removes the stdin writer on disconnect, but only if it is still the
// one we registered - so a stale terminal tearing down cannot yank the stdin out
// from under a newer one that already reconnected.
func (h *termHub) clearStdin(w io.Writer) {
	h.mu.Lock()
	if h.stdin == w {
		h.stdin = nil
	}
	h.mu.Unlock()
}

// writeStdin sends bytes to the live shell, or ErrNoShell if none is attached.
func (h *termHub) writeStdin(p []byte) error {
	h.mu.Lock()
	w := h.stdin
	h.mu.Unlock()
	if w == nil {
		return ErrNoShell
	}
	_, err := w.Write(p)
	return err
}

// broadcast records output to the ring and fans it to subscribers. Non-blocking
// per subscriber: a slow AI reader drops frames rather than stalling the
// student's terminal output.
//
// Output also counts as activity for the idle reaper. Every other activity
// signal is an INPUT path (typing, AI exec, context poll, browser keepalive),
// which only marks the session at the moment work is *launched*: AIExec bumps
// activity as it writes the command and returns within seconds, so a 90-minute
// fuzz or kernel build ran for an hour with the reaper seeing a perfectly idle
// session and killing it mid-job, long before the hard cap. A shell producing
// output is by definition not idle, so the output path marks it too - throttled,
// and outside h.mu so the manager lock is never taken under the hub lock.
func (h *termHub) broadcast(p []byte) {
	h.mu.Lock()
	h.ring = append(h.ring, p...)
	if len(h.ring) > scrollbackCap {
		h.ring = h.ring[len(h.ring)-scrollbackCap:]
	}
	for _, ch := range h.subs {
		select {
		case ch <- append([]byte(nil), p...):
		default:
		}
	}
	mark := h.onOutput
	if mark != nil {
		if now := time.Now(); now.Sub(h.lastOutputPing) >= outputActivityThrottle {
			h.lastOutputPing = now
		} else {
			mark = nil // throttled: someone already marked activity this interval
		}
	}
	h.mu.Unlock()
	if mark != nil {
		mark()
	}
}

func (h *termHub) subscribe() (int, chan []byte) {
	h.mu.Lock()
	id := h.nextSub
	h.nextSub++
	ch := make(chan []byte, 256)
	h.subs[id] = ch
	h.mu.Unlock()
	return id, ch
}

func (h *termHub) unsubscribe(id int) {
	h.mu.Lock()
	if ch, ok := h.subs[id]; ok {
		delete(h.subs, id)
		close(ch)
	}
	h.mu.Unlock()
}

func (h *termHub) scrollback() []byte {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]byte(nil), h.ring...)
}

// hasShell reports whether a student terminal is currently attached.
func (h *termHub) hasShell() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stdin != nil
}

// hubWriter tees exec stdout into a hub (alongside the student's websocket).
type hubWriter struct{ h *termHub }

func (w hubWriter) Write(p []byte) (int, error) {
	w.h.broadcast(p)
	return len(p), nil
}

// aiGrant is a live, revocable capability for the AI channel on one session.
type aiGrant struct {
	jti     string
	expires time.Time
}

// execRecord is the last command the AI ran and the raw output it produced, kept
// so /ai/context can hand the AI a clean structured record instead of making it
// re-scrape the terminal scrollback.
type execRecord struct {
	command string
	output  string // raw PTY bytes; the API layer cleans them for the response
	at      time.Time
}

// aiState holds all AI plumbing, keyed by session id, under one lock.
type aiState struct {
	mu       sync.Mutex
	grants   map[string]aiGrant
	hubs     map[string]*termHub
	tabs     map[string]string     // last tab the student had open, reported by the UI
	guide    map[string]guidePos   // last course step the student was on, reported by the UI
	lastExec map[string]execRecord // last AI-run command + output, per session
}

// guidePos is the student's live spot in the guided course (0-based index within
// total steps), so the AI can pace with them rather than leaping to the capture.
type guidePos struct {
	index int
	total int
}

func newAIState() *aiState {
	return &aiState{
		grants:   map[string]aiGrant{},
		hubs:     map[string]*termHub{},
		tabs:     map[string]string{},
		guide:    map[string]guidePos{},
		lastExec: map[string]execRecord{},
	}
}

func (m *Manager) setLastExec(sessionID, command, output string) {
	m.ai.mu.Lock()
	m.ai.lastExec[sessionID] = execRecord{command: command, output: output, at: time.Now()}
	m.ai.mu.Unlock()
}

// LastExec returns the last command the AI ran in this session and its raw
// output. ok is false if the AI hasn't run anything yet.
func (m *Manager) LastExec(sessionID string) (command, output string, at time.Time, ok bool) {
	m.ai.mu.Lock()
	defer m.ai.mu.Unlock()
	r, ok := m.ai.lastExec[sessionID]
	return r.command, r.output, r.at, ok
}

// SetActiveTab records which lab tab the student is currently viewing
// (terminal/topology/ai/guide/defender), reported by the frontend, so the AI can
// tailor "where you are" to what the student is actually looking at.
func (m *Manager) SetActiveTab(sessionID, tab string) {
	m.ai.mu.Lock()
	m.ai.tabs[sessionID] = tab
	m.ai.mu.Unlock()
}

// ActiveTab returns the student's last-reported tab, or "" if unknown.
func (m *Manager) ActiveTab(sessionID string) string {
	m.ai.mu.Lock()
	defer m.ai.mu.Unlock()
	return m.ai.tabs[sessionID]
}

// SetGuideStep records which course step the student is on (0-based index within
// total), reported by the frontend, so the AI can meet them at their place in
// the guided course instead of racing ahead to the final capture.
func (m *Manager) SetGuideStep(sessionID string, index, total int) {
	m.ai.mu.Lock()
	m.ai.guide[sessionID] = guidePos{index: index, total: total}
	m.ai.mu.Unlock()
}

// GuideStep returns the student's last-reported course position. ok is false if
// the UI hasn't reported one yet.
func (m *Manager) GuideStep(sessionID string) (index, total int, ok bool) {
	m.ai.mu.Lock()
	defer m.ai.mu.Unlock()
	g, ok := m.ai.guide[sessionID]
	return g.index, g.total, ok
}

// hubFor returns the session's terminal hub, creating it on first use.
func (m *Manager) hubFor(sessionID string) *termHub {
	m.ai.mu.Lock()
	defer m.ai.mu.Unlock()
	h := m.ai.hubs[sessionID]
	if h == nil {
		h = newTermHub()
		// Shell output keeps the session's idle clock alive - see broadcast. Safe
		// to close over the manager: Activity takes m.mu, a different lock from the
		// m.ai.mu held here, and broadcast calls this outside the hub lock.
		h.onOutput = func() { m.Activity(sessionID) }
		m.ai.hubs[sessionID] = h
	}
	return h
}

// GrantAI issues a fresh AI grant, replacing any prior one - so re-issuing
// immediately invalidates the previous jti. Returns the expiry.
func (m *Manager) GrantAI(sessionID, jti string, ttl time.Duration) time.Time {
	exp := time.Now().Add(ttl)
	m.ai.mu.Lock()
	m.ai.grants[sessionID] = aiGrant{jti: jti, expires: exp}
	m.ai.mu.Unlock()
	return exp
}

// RevokeAI drops any AI grant for a session (panel closed, or teardown).
func (m *Manager) RevokeAI(sessionID string) {
	m.ai.mu.Lock()
	delete(m.ai.grants, sessionID)
	m.ai.mu.Unlock()
}

// extendGrant slides a live AI grant's expiry out by `by`, bounded at `ceiling`,
// WITHOUT re-minting - the jti and the researcher's pasted token stay valid, so
// an AI extending mid-work keeps using the same token. For research labs the
// JWT's own exp is issued out to the 48h ceiling at mint time (see aiGrantCreate)
// precisely so this in-place bump is all that is needed. No-op if there is no
// grant. Called by Manager.Extend, which may hold m.mu (lock order m.mu ->
// m.ai.mu, matching deleteWithReason).
func (m *Manager) extendGrant(sessionID string, by time.Duration, ceiling time.Time) {
	m.ai.mu.Lock()
	defer m.ai.mu.Unlock()
	g, ok := m.ai.grants[sessionID]
	if !ok {
		return
	}
	exp := g.expires.Add(by)
	if exp.After(ceiling) {
		exp = ceiling
	}
	g.expires = exp
	m.ai.grants[sessionID] = g
}

// GrantExpiry returns the live AI grant's expiry for a session, if one exists -
// so /ai/context can tell the AI how long its OWN channel has left, separately
// from the session's hard cap (the two move together on Extend, but a grant can
// also be minted late in a session's life).
func (m *Manager) GrantExpiry(sessionID string) (time.Time, bool) {
	m.ai.mu.Lock()
	defer m.ai.mu.Unlock()
	g, ok := m.ai.grants[sessionID]
	if !ok {
		return time.Time{}, false
	}
	return g.expires, true
}

// VerifyAIGrant reports whether jti is the session's current, unexpired grant.
// Expiry here is the belt to the JWT's suspenders: revocation and the 15m cap
// both flow through this one check, so the earliest of the two always wins.
func (m *Manager) VerifyAIGrant(sessionID, jti string) bool {
	m.ai.mu.Lock()
	defer m.ai.mu.Unlock()
	g, ok := m.ai.grants[sessionID]
	if !ok || g.jti != jti {
		return false
	}
	if time.Now().After(g.expires) {
		delete(m.ai.grants, sessionID) // opportunistic cleanup past expiry
		return false
	}
	return true
}

// AIExec writes one command to the session's shared shell (as if typed) and
// returns the output it produced, collected until the shell goes quiet or a cap
// is hit. ErrNoShell if the student has no terminal open - the AI cannot run
// unseen. Marks activity so an AI-driven session is not idle-reaped mid-task.
func (m *Manager) AIExec(sessionID, command string) (string, error) {
	h := m.hubFor(sessionID)
	if !h.hasShell() {
		return "", ErrNoShell
	}
	id, ch := h.subscribe()
	defer h.unsubscribe(id)
	if err := h.writeStdin([]byte(command + "\n")); err != nil {
		return "", err
	}
	m.Activity(sessionID)

	// Collect the command's output. The reliable "done" signal is the shell
	// PROMPT returning after some output (execDone) - return as soon as we see it.
	// The timers are only fallbacks: a longer quiet window than before (a kubectl
	// call sits silent for ~1s between the echoed keystrokes and its result, and
	// the old 700ms fired in that gap and returned just the echo), plus a hard
	// ceiling and a size cap for a chatty or hung command.
	var buf []byte
	quiet := time.NewTimer(1500 * time.Millisecond)
	defer quiet.Stop()
	hard := time.NewTimer(10 * time.Second)
	defer hard.Stop()
collect:
	for {
		select {
		case p, ok := <-ch:
			if !ok {
				break collect
			}
			buf = append(buf, p...)
			if execDone(buf, command) || len(buf) >= 64*1024 {
				break collect
			}
			if !quiet.Stop() {
				select {
				case <-quiet.C:
				default:
				}
			}
			quiet.Reset(1500 * time.Millisecond)
		case <-quiet.C:
			break collect
		case <-hard.C:
			break collect
		}
	}
	out := string(buf)
	m.setLastExec(sessionID, command, out) // so /ai/context can surface it structured
	return out, nil
}

// execAnsi strips terminal control sequences for the execDone prompt check.
var execAnsi = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]|\x1b\\][^\x07\x1b]*(?:\x07|\x1b\\\\)|\x1b[@-Z\\\\-_]")

// execDone reports whether the shell prompt has returned after the command ran -
// i.e. the command finished. It looks past the echoed command line for output
// followed by a trailing prompt sigil (# for root, $ otherwise). Returns false
// until it has seen the echo AND at least one output line, so it never fires on
// the pre-command prompt. If the echo is line-wrapped (narrow PTY) and the
// command can't be located, this stays false and AIExec falls back to its timers.
func execDone(raw []byte, command string) bool {
	s := execAnsi.ReplaceAllString(string(raw), "")
	i := strings.Index(s, command)
	if i < 0 {
		return false
	}
	after := s[i+len(command):]
	if !strings.Contains(after, "\n") {
		return false
	}
	t := strings.TrimRight(after, " \t\r\n")
	if t == "" {
		return false
	}
	last := t[len(t)-1]
	return last == '#' || last == '$'
}

// AIScrollback returns the session's recent terminal output for /ai/context.
func (m *Manager) AIScrollback(sessionID string) string {
	return string(m.hubFor(sessionID).scrollback())
}

// AIHasShell reports whether the student's terminal is currently open (and thus
// whether the AI can act at all).
func (m *Manager) AIHasShell(sessionID string) bool {
	return m.hubFor(sessionID).hasShell()
}

// dropHub removes a session's hub (called on teardown). Any AI subscribers see
// their channels close and their handlers exit.
func (m *Manager) dropHub(sessionID string) {
	m.ai.mu.Lock()
	h := m.ai.hubs[sessionID]
	delete(m.ai.hubs, sessionID)
	delete(m.ai.tabs, sessionID)
	delete(m.ai.lastExec, sessionID)
	m.ai.mu.Unlock()
	if h != nil {
		h.mu.Lock()
		for id, ch := range h.subs {
			delete(h.subs, id)
			close(ch)
		}
		h.mu.Unlock()
	}
}
