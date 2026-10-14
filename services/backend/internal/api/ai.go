package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"backend/internal/auth"
	"backend/internal/course"
	"backend/internal/labs"
	"backend/internal/topology"
)

// The AI lab-assist surface. A student can grant an external AI (their own
// Claude/agent, via a pasted config) a short-lived, tightly-scoped window into
// ONE running lab session: read the lesson/topology/terminal, and run commands
// that appear on the student's own terminal. It deliberately cannot submit
// flags, start/stop the lab, or touch anything outside the session - those are
// enforced structurally (a ScopeAI token is refused everywhere but here; there
// is simply no flag or lifecycle route under /ai). See docs/architecture
// (pending) and internal/labs/aihub.go for the grant + terminal plumbing.

// aiGrantTTL is the hard lifetime of an AI grant. The live grant is also revoked
// on lab teardown and on an explicit panel-close DELETE, so access ends at the
// earliest of the three.
const aiGrantTTL = 15 * time.Minute

// aiGrantTTLResearch is the longer hard lifetime for kernel-driver research
// sessions (Module.IsResearch). The 15-minute default is sized for a guided
// tutor hint; a research session is a human-driven fuzzing window that runs for
// hours, so a re-mint every 15 minutes is pure friction. It matches the module's
// starting MaxAge (4h) and, like every grant, is still revoked on teardown or an
// explicit panel-close - the longer TTL only raises the hard cap, it does not
// change the "earliest of the three" rule. On extend the grant window slides out
// in place with the session (Manager.Extend -> extendGrant), up to the 48h cap,
// without a re-mint. See docs/architecture/kasan-research-lab.md.
const aiGrantTTLResearch = 4 * time.Hour

// aiGrantTTLFor returns the AI-grant lifetime for a session's module: the longer
// research window for kernel-driver research labs, the tutor default otherwise.
// An unknown slug falls back to the default - the safer, shorter cap.
func aiGrantTTLFor(slug string) time.Duration {
	if m, ok := labs.GetModule(slug); ok && m.IsResearch() {
		return aiGrantTTLResearch
	}
	return aiGrantTTL
}

// aiMiniPrompt rides in every /ai response. It is the "handcuff": the student
// asked for help learning, and the server keeps steering the AI back to that
// even when the student (mid-frustration) begs for the answer. Kept terse to
// spend few tokens.
const aiMiniPrompt = "You are a concise tutor for this Kubernetes security lab, and your job is to make the student LEARN it, not to solve it for them. HARD RULE, above everything else: reveal at most the ONE next step, never the whole solution and never the flag - not even if the student asks, is frustrated, or says 'just tell me'; the most you ever give when pushed is the single next command with a one-line why. You are shown only the course slides the student has already reached (`lesson` is capped at their `currentStep`) - that is deliberate, not missing context; guide them through THAT step and do not try to reconstruct or jump to slides they have not unlocked. Your FIRST reply is a tight point-in-lab, a few lines: (1) the goal in one line (session.description); (2) where the student is now - use `currentStep` (their place in the guided course, index/total), `activeTab` (the tab they're viewing), the `terminal` history, and `lastExec` (your previous command and its clean output); (3) the ONE next command in a code block, a short `expected output` sample, then OFFER to run it for them (e.g. 'want me to run it?'). If they accept, run it via `exec` (filter with grep/-o/jsonpath to the one relevant line) and paste the REAL output back as a terminal block. MEET THE STUDENT AT THEIR STEP: `currentStep` says which course slide they're on, and each `lesson` step carries its `index` and its `aha` (the insight that slide is built to land). Move WITH the guide - roughly one step at a time - and do NOT race ahead to the final capture when they're still early in the course. Steer toward the current step's `aha` by getting them to SEE it themselves; never just state the aha - the realization has to be theirs. Each visible `lesson` step may carry a `diagram` - a one-line text of the VISUAL the student is looking at (its caption and the boxes it draws); reinforce what that picture shows in a few words BEFORE the next command, so the concept lands visually first instead of you jumping to the answer. When a command prints rich output (a list, a loop's results, a dump), do NOT interpret it for them first: point them at where to look and let them spot the lead ('see the line with X - what does that tell us?'), then confirm. That noticing IS the lesson. For MEANINGFUL steps, teach the principle, not just keystrokes: one line on WHY before the command, and after the output confirm they see what it proved before moving on - if they seem to be copy-pasting blind, ask a quick check first. Be a bit stubborn - it's fine to make them engage with the concept to earn the next command - but not pushy: one nudge, and if they just want to move, give the next command (only that one) with the one-line why attached. Skip all this on trivial/rote commands (ls, cd, clear) - don't lecture. Keep EVERY reply short - no greetings, filler, or recaps. Work ONE step at a time. Never print or submit the flag - the final capture is the student's. You cannot start/stop the lab; everything you exec shows live on their terminal."

// aiResearchPrompt is the mini-prompt for the kernel-driver research labs
// (Module.IsResearch). These are not a guided, hide-the-answer course: the
// session is a disposable, root-in-a-throwaway-microVM kernel-research bench with
// no flag and no solution to withhold, so the tutor "handcuff" above is the wrong
// mode. This is a plain, generic research-assistant prompt instead.
const aiResearchPrompt = "You are a kernel-security research assistant with live, mirrored access to a disposable Linux kernel-research microVM. The instrumented guest kernel (KASAN, KCOV, BTF, kprobes) is itself what you research; the session already runs as root inside a throwaway VM, so there is no flag, no 'solution' to withhold, and no privilege to protect - help the researcher do real kernel-driver security work. Read the session context with GET /context (it carries session.description, the active tab, and recent terminal output) and run commands in the researcher's live terminal with POST /exec - which they watch on their own Terminal tab. Typical work: build and insmod a driver, fuzz its ioctl surface, read and interpret KASAN reports from dmesg, reason about slab layout and reclaim, and develop an exploit (e.g. a data-only cred overwrite). Be concrete and technical: cite the exact commands and the real output, keep replies tight, and go as deep as the researcher wants. You cannot start or stop the lab."

// aiDefenderLine is appended (to either prompt above) for labs that ship an
// in-guest eBPF sensor (Module.HasDefender). It tells the AI it can actually
// SEE the kernel-event feed - `defenderEvents` in GET /context - so the "you
// have eBPF access" claim is backed by a real, readable field, not a promise.
const aiDefenderLine = "This lab also has a live eBPF sensor (Tetragon): the most recent kernel events it captured - process exec, and commit_creds credential changes with their uid/euid - ride in `defenderEvents` in GET /context, so you can watch the kernel's behaviour alongside the terminal (note a data-only cred overwrite never calls commit_creds, which is exactly why it is stealthy)."

// aiExtendLine is appended for the extendable research labs. They start on a
// deliberately short clock so idle sessions free compute; the AI is expected to
// keep its own session alive by extending when it is genuinely mid-task, and to
// let it lapse otherwise.
const aiExtendLine = "This session runs on TWO clocks and dies to whichever fires first. (1) An idle timer, `idleWindowSeconds`: it resets when you start a command, when the shell produces output, and on every GET /context poll. (2) A hard cap, `hardCapExpiresInSeconds`, which nothing resets - only POST /extend moves it. `expiresInSeconds` is the soonest of the two. IMPORTANT: starting a command resets the idle timer once, at launch - it does not hold the session open for the command's whole run. A job that runs silently for longer than `idleWindowSeconds` (a long build, a fuzz run, a sleep) can therefore idle the session out from under itself; the shell's own output covers most long jobs, but if a job may go quiet for longer than the idle window, POST /extend first or poll GET /context while it runs. POST /extend (Authorization: Bearer <token>) adds 2h and resets both clocks, up to a 48h hard cap; your own access slides out with the session. Let it lapse when you are done so idle labs free compute for others."

// aiMiniPromptFor returns the mini-prompt for a session's module: the generic
// research-assistant prompt for kernel-driver research labs, the guided tutor
// prompt otherwise. An unknown slug falls back to the tutor prompt.
func aiMiniPromptFor(slug string) string {
	m, ok := labs.GetModule(slug)
	if !ok {
		return aiMiniPrompt
	}
	base := aiMiniPrompt
	if m.IsResearch() {
		base = aiResearchPrompt
		// Only the research labs are extendable (Manager.Extend gates on
		// IsResearch), so the self-extend instructions ride only their prompt.
		base += " " + aiExtendLine
	}
	// eBPF access is per-module (HasDefender), which cuts across both the guided
	// and research labs, so append it to whichever base prompt applies.
	if m.HasDefender {
		base += " " + aiDefenderLine
	}
	return base
}

// aiGrantResponse is the paste-ready config the student hands to their AI.
type aiGrantResponse struct {
	SessionID  string            `json:"sessionId"`
	Token      string            `json:"token"`
	ExpiresAt  time.Time         `json:"expiresAt"`
	TTLSeconds int               `json:"ttlSeconds"`
	BaseURL    string            `json:"baseUrl"`
	MiniPrompt string            `json:"miniPrompt"`
	API        map[string]string `json:"api"`
}

// ansiSeq matches the terminal control sequences in a raw PTY stream: CSI
// (colors, cursor moves, clears), OSC (title sets), and lone two-byte escapes.
var ansiSeq = regexp.MustCompile("\x1b\\[[0-9;?]*[ -/]*[@-~]|\x1b\\][^\x07\x1b]*(?:\x07|\x1b\\\\)|\x1b[@-Z\\\\-_]")

// cleanTerminal turns the raw PTY scrollback into readable text for the AI's
// /context view. The raw stream is ANSI colors plus cursor-movement/redraw
// sequences hard-wrapped to the terminal width - unreadable to a model. We only
// clean it here; the live /ai/terminal websocket still streams the raw bytes,
// which xterm needs to render. Carriage returns and other control bytes are
// dropped (newlines and tabs kept), which flattens the line-edit redraws into
// plain command+output lines - good enough for "what did the student run".
func cleanTerminal(s string) string {
	s = ansiSeq.ReplaceAllString(s, "")
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	// Collapse the runs of blank lines the redraw-flattening leaves behind.
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// newJTI returns a random token id used to revoke a grant out-of-band.
func newJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// aiBaseURL builds the absolute /ai base for this session from the incoming
// request, so the pasted config points at wherever the caller reached us.
func aiBaseURL(r *http.Request, sessionID string) string {
	scheme := "http"
	if isHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host + "/api/v1/ai/sessions/" + sessionID
}

// aiGrantCreate mints (or re-mints) an AI grant for a session the caller owns.
// Re-minting invalidates any previous grant (the store keeps only the newest
// jti), so "regenerate" instantly kills the old key.
func aiGrantCreate(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		sess, ok := mgr.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		userID, _ := r.Context().Value(userIDKey).(string)
		if sess.OwnerID != userID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		jti, err := newJTI()
		if err != nil {
			http.Error(w, "failed to mint grant", http.StatusInternalServerError)
			return
		}
		// The grant store's `expires` (what VerifyAIGrant actually checks) is the
		// reported window and the TTLSeconds we return. The JWT's exp normally
		// matches it - EXCEPT for the extendable research labs, where it is issued
		// out to the 48h hard cap so Manager.Extend can slide the store window in
		// place (up to 48h) WITHOUT re-minting, and the researcher's pasted token
		// keeps working across extends. The store expiry still bounds real access.
		ttl := aiGrantTTLFor(sess.ModuleSlug)
		jwtTTL := ttl
		if m, ok := labs.GetModule(sess.ModuleSlug); ok && m.IsResearch() {
			if d := time.Until(sess.CreatedAt.Add(labs.ExtendHardCap)); d > jwtTTL {
				jwtTTL = d
			}
		}
		token, err := auth.IssueAIToken(userID, id, jti, jwtTTL)
		if err != nil {
			http.Error(w, "failed to issue token", http.StatusInternalServerError)
			return
		}
		exp := mgr.GrantAI(id, jti, ttl)
		base := aiBaseURL(r, id)
		api := map[string]string{
			"auth":     "send Authorization: Bearer <token> on every call",
			"context":  "GET " + base + "/context  -> lesson, topology, and recent terminal output",
			"exec":     "POST " + base + "/exec  {\"command\":\"...\"}  -> {\"output\":\"...\"}; runs in the student's live terminal (visible to them)",
			"terminal": "WS " + base + "/terminal  binary stdin/stdout, same protocol as the browser terminal",
		}
		// Only the extendable research labs advertise /extend; the guided tutor
		// labs have no extend and never surface the endpoint.
		if m, ok := labs.GetModule(sess.ModuleSlug); ok && m.IsResearch() {
			api["extend"] = "POST " + base + "/extend  -> {\"expiresAt\":\"...\",\"expiresInSeconds\":N}; adds 2h to this session (up to a 48h cap) when you are still mid-task"
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(aiGrantResponse{
			SessionID:  id,
			Token:      token,
			ExpiresAt:  exp,
			TTLSeconds: int(ttl.Seconds()),
			BaseURL:    base,
			MiniPrompt: aiMiniPromptFor(sess.ModuleSlug),
			API:        api,
		})
	}
}

// aiGrantRevoke drops the session's AI grant (the student closed the panel).
func aiGrantRevoke(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		sess, ok := mgr.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		userID, _ := r.Context().Value(userIDKey).(string)
		if sess.OwnerID != userID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		mgr.RevokeAI(id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// aiSetActiveTab records which lab tab the owner is currently viewing, so
// /ai/context can tell the AI where the student's attention is. Ownership-gated
// like the rest; a no-op body value just clears the hint.
func aiSetActiveTab(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		sess, ok := mgr.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		userID, _ := r.Context().Value(userIDKey).(string)
		if sess.OwnerID != userID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body struct {
			Tab string `json:"tab"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "expected {\"tab\":\"...\"}", http.StatusBadRequest)
			return
		}
		mgr.SetActiveTab(id, body.Tab)
		w.WriteHeader(http.StatusNoContent)
	}
}

// aiSetGuideStep records which course step the owner is currently on, so
// /ai/context can tell the AI where the student is in the guided course and the
// tutor can pace with them instead of jumping to the final capture. Ownership-
// gated like aiSetActiveTab; best-effort from the frontend.
func aiSetGuideStep(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		sess, ok := mgr.Get(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		userID, _ := r.Context().Value(userIDKey).(string)
		if sess.OwnerID != userID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var body struct {
			Index int `json:"index"`
			Total int `json:"total"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "expected {\"index\":N,\"total\":N}", http.StatusBadRequest)
			return
		}
		mgr.SetGuideStep(id, body.Index, body.Total)
		w.WriteHeader(http.StatusNoContent)
	}
}

// requireAIGrant gates the /ai/* routes. It accepts ONLY a ScopeAI bearer token
// (never a cookie), bound to the session in the path, whose jti is the session's
// current live grant, for a session that is still running. Any failure is a flat
// 401/404 - it never falls through to user auth, and a ScopeAI token is refused
// by requireAuth everywhere else, so this is the only door the AI key opens.
func requireAIGrant(mgr *labs.Manager, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		authz := r.Header.Get("Authorization")
		const p = "Bearer "
		if len(authz) <= len(p) || authz[:len(p)] != p {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		claims, err := auth.VerifyToken(authz[len(p):])
		if err != nil || auth.Scope(claims) != auth.ScopeAI {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		sid, _ := claims["sid"].(string)
		jti, _ := claims["jti"].(string)
		if sid == "" || sid != id {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !mgr.VerifyAIGrant(sid, jti) {
			// Revoked, expired, or superseded by a newer grant.
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if _, live := mgr.Get(id); !live {
			// Lab ended out from under the grant.
			http.Error(w, "session ended", http.StatusGone)
			return
		}
		next(w, r)
	}
}

// aiLessonStep is one course step, raw markdown, for the AI to read. Index is
// the step's position (matches currentStep.index), so the AI can line up "the
// slide they're on" with its content. Aha is that step's intended insight - the
// AI uses it as the target to steer the student toward, NOT as a line to recite
// (the prompt forbids stating it outright).
type aiLessonStep struct {
	Index int    `json:"index"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Aha   string `json:"aha,omitempty"`
	// Diagram is a compact text of the VISUAL on this slide (the student is
	// looking at an animated SVG the AI can't render). Handing the AI the
	// picture's content lets the tutor reinforce it in words before a command -
	// the "show the visual before the solution" the reporter asked for.
	Diagram string `json:"diagram,omitempty"`
}

// diagramSummary flattens a step's declarative diagram into one text line for the
// AI: the caption plus the boxes/labels it draws. Coordinates are dropped - a
// model needs the nouns and the caption's relationship, not the geometry - and
// wires (point-to-point, not box-to-box) carry no names to extract, so we skip
// them. Empty when the step has no diagram.
func diagramSummary(d *course.Diagram) string {
	if d == nil {
		return ""
	}
	var nodes []string
	for _, s := range d.Shapes {
		switch s.Type {
		case "box":
			if s.Label == "" {
				continue
			}
			if s.Sub != "" {
				nodes = append(nodes, s.Label+" ("+s.Sub+")")
			} else {
				nodes = append(nodes, s.Label)
			}
		case "label":
			if s.Text != "" {
				nodes = append(nodes, s.Text)
			}
		}
	}
	var b strings.Builder
	if d.Caption != "" {
		b.WriteString(d.Caption)
	}
	if len(nodes) > 0 {
		if b.Len() > 0 {
			b.WriteString(" | ")
		}
		b.WriteString("shows: " + strings.Join(nodes, "; "))
	}
	return b.String()
}

// aiGuidePos is the student's live position in the guided course, so the AI can
// pace with them instead of leaping to the capture.
type aiGuidePos struct {
	Index int `json:"index"`
	Total int `json:"total"`
}

// aiLastExec is a clean, structured record of the last command the AI ran and
// its output - so the AI doesn't have to re-scrape the wrapped `terminal`
// scrollback to see what its own exec produced.
type aiLastExec struct {
	Command string `json:"command"`
	Output  string `json:"output"`
	At      string `json:"at"`
}

// aiContextResponse is the read-only view the AI gets of the current session.
// Ready is the agent's liveness signal: false while the lab is still booting (or
// the student's terminal isn't attached yet), true once exec will actually run.
// An agent can be handed a key during provisioning and poll /context until ready.
//
// The guardrail miniPrompt is deliberately NOT repeated here (the AI already
// holds it from the pasted config, and /ai/exec still returns it on every
// action - the place the "don't hand over the answer" reminder matters most).
type aiContextResponse struct {
	Ready       bool                     `json:"ready"`
	ActiveTab   string                   `json:"activeTab,omitempty"`
	CurrentStep *aiGuidePos              `json:"currentStep,omitempty"`
	Session     map[string]string        `json:"session"`
	Lesson      []aiLessonStep           `json:"lesson"`
	Topology    *topology.ScopedTopology `json:"topology"`
	Terminal    string                   `json:"terminal"`
	LastExec    *aiLastExec              `json:"lastExec,omitempty"`
	// DefenderEvents is the tail of the in-guest eBPF sensor's feed (Tetragon) for
	// labs that ship one (Module.HasDefender) - the same live stream the Defender
	// pane renders, condensed for the model so it can observe the kernel's
	// behaviour alongside the terminal. Empty/absent when the lab has no sensor,
	// isn't ready, or the tail is momentarily unreachable.
	DefenderEvents []string `json:"defenderEvents,omitempty"`
	// ExpiresInSeconds is how long the AI's own working channel has left: the
	// seconds until the SOONEST of the session's idle deadline, its hard cap, and
	// the grant's expiry. Served whenever the session is running. For the
	// extendable research labs the AI is told to POST /extend before this reaches
	// zero if it is still mid-task; see aiExtendLine. Omitted (0) once past the
	// deadline, when the teardown is imminent anyway.
	//
	// The two clocks below are reported separately because the single minimum is
	// ambiguous in the case that actually kills long jobs. Every context poll
	// resets the idle clock, so ExpiresInSeconds reads ~IdleWindowSeconds at every
	// poll while the cap is far away - the AI could not tell "60m of idle window,
	// which my long-running job will NOT refresh" from "60m of session left", and
	// so never knew a 90-minute build would outlive the timer.
	ExpiresInSeconds int `json:"expiresInSeconds,omitempty"`
	// IdleWindowSeconds is the full idle window: how long the session survives
	// with no activity at all, measured from the last activity. Compare a job's
	// expected runtime against THIS to decide whether it needs a heartbeat.
	IdleWindowSeconds int `json:"idleWindowSeconds,omitempty"`
	// HardCapExpiresInSeconds is the seconds until the immovable hard cap, which
	// only POST /extend moves - no amount of activity slides it.
	HardCapExpiresInSeconds int               `json:"hardCapExpiresInSeconds,omitempty"`
	Notes                   map[string]string `json:"notes,omitempty"`
}

// aiContext returns the lesson, the student's scoped topology, and the recent
// terminal output - the same material the student is looking at.
func aiContext(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		// requireAIGrant confirmed the session was live, but it can be torn down
		// between that check and here - guard against the nil rather than deref it.
		sess, ok := mgr.Get(id)
		if !ok {
			http.Error(w, "session ended", http.StatusGone)
			return
		}

		// An AI reading its context is the researcher actively engaging this
		// session through their assistant - even a discussion turn that runs no
		// command. Count it as activity so an AI-assisted session (which can go many
		// minutes between exec calls while the AI and researcher reason about a KASAN
		// dump or an exploit) is not idle-reaped mid-conversation. Exec and terminal
		// input already reset the idle clock; context reads were the gap that let a
		// talky-but-active session lapse to "Session closed". No-op if already gone.
		mgr.Activity(id)

		resp := aiContextResponse{
			ActiveTab: mgr.ActiveTab(id),
			Session: map[string]string{
				"id":     id,
				"module": sess.ModuleSlug,
				"status": sess.Status,
			},
			Terminal: cleanTerminal(mgr.AIScrollback(id)),
		}
		if module, ok := labs.GetModule(sess.ModuleSlug); ok {
			resp.Session["moduleName"] = module.DisplayName
			resp.Session["description"] = module.Description
			resp.Session["hint"] = module.Hint
		}

		// Where the student is in the guided course, so the AI paces with them
		// (meet them at their step) rather than leaping to the capture.
		if idx, total, ok := mgr.GuideStep(id); ok {
			resp.CurrentStep = &aiGuidePos{Index: idx, Total: total}
		}

		// Lesson: the course steps the student has actually reached, as raw
		// markdown (best for a model to read), each tagged with its index and its
		// aha (the insight to steer toward). We CAP at the student's current step
		// (currentStep.index) on purpose. The student reads the course one slide at
		// a time and cannot see the ending until they page to it; the un-capped
		// lesson instead handed the AI the entire walkthrough - technique, fix, and
		// capture - the moment it read /context, so a helpful-but-eager tutor would
		// synthesize and print the whole solution while the student was still early,
		// even without being asked. Capping to `<= currentStep` gives the AI only
		// what the student has unlocked, so it literally cannot read ahead. Fallback
		// when the student's position is unknown (guide-step not reported yet): the
		// opening step alone, never the full solution.
		if c, ok := course.Get(sess.ModuleSlug); ok {
			maxIdx := 0
			if resp.CurrentStep != nil {
				maxIdx = resp.CurrentStep.Index
			}
			for _, s := range c.Steps {
				if s.Index > maxIdx {
					continue
				}
				resp.Lesson = append(resp.Lesson, aiLessonStep{Index: s.Index, Title: s.Title, Body: s.Body, Aha: s.Aha, Diagram: diagramSummary(s.Diagram)})
			}
		}

		// Topology: the student's own scoped map, if the nested kubeconfig is
		// cached yet. spoilers=true matches exactly what the student sees.
		if kc, cached := mgr.NestedKubeconfigCached(id); cached {
			topo := topology.Scoped(r.Context(), kc, true)
			if module, ok := labs.GetModule(sess.ModuleSlug); ok && module.Nested != nil {
				if ap := module.Nested.AttackPath; ap != nil {
					topo.Edges = append(topo.Edges, topology.Edge{From: ap.From, To: ap.To, Port: ap.Label, Kind: "escape"})
				}
			}
			resp.Topology = topo
		}

		if cmd, out, at, ok := mgr.LastExec(id); ok {
			resp.LastExec = &aiLastExec{Command: cmd, Output: cleanTerminal(out), At: at.UTC().Format(time.RFC3339)}
		}

		// For labs that ship an in-guest eBPF sensor, hand the AI the same live
		// feed the Defender pane shows (condensed). Best-effort and only once the
		// session is up: a fetch failure just omits the field rather than failing
		// the whole context poll.
		if sess.Status == "running" {
			if events, err := mgr.DefenderRecent(r.Context(), sess, 30); err != nil {
				slog.Debug("ai context: defender tail", "session", id, "error", err)
			} else if len(events) > 0 {
				resp.DefenderEvents = events
			}
		}

		resp.Ready = sess.Status == "running" && mgr.AIHasShell(id)
		if !resp.Ready {
			if sess.Status != "running" {
				resp.Notes = map[string]string{"lab": "the lab is still starting; keep reading the lesson and poll until ready is true, then exec will work"}
			} else {
				resp.Notes = map[string]string{"terminal": "the student's terminal is not attached yet; exec will 409 until ready is true"}
			}
		}

		// How long the AI's own channel has left: the earlier of the session's hard
		// cap and the grant's expiry (they slide together on Extend, but a grant
		// minted late in the session's life can be the tighter bound). This is what
		// the self-extend warning counts down to.
		if sess.Status == "running" {
			deadline := sess.ExpiresAt()
			// The idle deadline is usually the SOONER of the session's two clocks and
			// is the one the AI must respect: reporting only the hard cap told the AI
			// it had hours left when an idle reap was minutes away, so it never knew to
			// keep the session warm. We just reset the idle clock above (this very
			// poll counts as activity), so the fresh idle deadline is now + the idle
			// window - measured from this interaction.
			if idle := time.Now().Add(sess.Timing.Idle); idle.Before(deadline) {
				deadline = idle
			}
			if gexp, ok := mgr.GrantExpiry(id); ok && gexp.Before(deadline) {
				deadline = gexp
			}
			if secs := int(time.Until(deadline).Seconds()); secs > 0 {
				resp.ExpiresInSeconds = secs
			}
			// The two clocks, unmerged, so the AI can size a job against the right one.
			resp.IdleWindowSeconds = int(sess.Timing.Idle / time.Second)
			if secs := int(time.Until(sess.ExpiresAt()).Seconds()); secs > 0 {
				resp.HardCapExpiresInSeconds = secs
			}
			// ~5 min out, nudge an extendable (research) lab's AI to extend if it is
			// still working. Guided tutor labs are not extendable, so they get no
			// extend nudge. The AI is expected to let an idle session simply lapse.
			if module, ok := labs.GetModule(sess.ModuleSlug); ok && module.IsResearch() {
				if d := time.Until(deadline); d > 0 && d <= 5*time.Minute {
					if resp.Notes == nil {
						resp.Notes = map[string]string{}
					}
					resp.Notes["expiry"] = "this session ends in under 5 minutes; POST /extend to add 2h and reset both clocks if you are still mid-task, otherwise let it lapse so idle compute frees up"
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// aiExec runs one command in the student's live terminal and returns its output.
func aiExec(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Command string `json:"command"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Command == "" {
			http.Error(w, "expected {\"command\":\"...\"}", http.StatusBadRequest)
			return
		}
		out, err := mgr.AIExec(id, body.Command)
		if err == labs.ErrNoShell {
			http.Error(w, "the student's terminal is not open; ask them to open it", http.StatusConflict)
			return
		}
		if err != nil {
			http.Error(w, "exec failed", http.StatusBadGateway)
			return
		}
		// The mini-prompt rides in every response so the steer sticks; pick the
		// one for this session's module (research vs guided tutor).
		prompt := aiMiniPrompt
		if sess, ok := mgr.Get(id); ok {
			prompt = aiMiniPromptFor(sess.ModuleSlug)
		}
		w.Header().Set("Content-Type", "application/json")
		// Clean the raw PTY bytes (ANSI, redraws) so the AI gets readable output.
		json.NewEncoder(w).Encode(map[string]string{"output": cleanTerminal(out), "miniPrompt": prompt})
	}
}

// aiExtend pushes an extendable (research) session's hard cap out by ExtendStep
// and slides the AI's own grant with it (Manager.Extend), returning the new
// deadline. It is the self-service knob the aiExtendLine prompt points the AI at
// ~5 min before timeout, and the researcher's own UI can drive the same handler.
// Grant-gated like the rest of /ai: it can prolong a session's life but still
// never submit a flag, stop the lab, or reach another session.
func aiExtend(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		newExp, err := mgr.Extend(id)
		if err != nil {
			switch {
			case errors.Is(err, labs.ErrSessionNotFound):
				http.Error(w, "session ended", http.StatusGone)
			case errors.Is(err, labs.ErrNotRunning):
				http.Error(w, err.Error(), http.StatusConflict)
			case errors.Is(err, labs.ErrNotExtendable):
				http.Error(w, err.Error(), http.StatusForbidden)
			case errors.Is(err, labs.ErrAtHardCap):
				http.Error(w, err.Error(), http.StatusConflict)
			default:
				http.Error(w, "failed to extend session", http.StatusInternalServerError)
			}
			return
		}
		secs := int(time.Until(newExp).Seconds())
		if secs < 0 {
			secs = 0
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"expiresAt":        newExp.UTC().Format(time.RFC3339),
			"expiresInSeconds": secs,
		})
	}
}

// aiTerminal bridges the AI to the shared shell over a websocket (same protocol
// as the browser terminal). See labs.Manager.AITerminalWS.
func aiTerminal(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		mgr.AITerminalWS(w, r, r.PathValue("id"))
	}
}
