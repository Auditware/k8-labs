package labs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Session struct {
	ID         string `json:"id"`
	ModuleSlug string `json:"moduleSlug"`
	Status     string `json:"status"` // "starting", "running", "error"
	Error      string `json:"error,omitempty"`
	// Phase is the last provisioning gate this session cleared while status is
	// "starting" (one of the phase() milestones in provisionNested: "pod-created",
	// "pod-ready", "kubeconfig-fetched", "nested-node-ready", "manifests-applied",
	// "foothold-ready"). Empty until the first gate clears, and meaningless once
	// running. The provisioning UI reads it to anchor its progress bar to real
	// backend progress instead of a wall-clock guess - so a slow (busy-hours)
	// boot no longer runs the bar ahead of reality. Written under Manager.mu by
	// the provision goroutine, read under it by Get.
	Phase string `json:"phase,omitempty"`
	// OwnerID is the creating user's ID - checked by the API layer before
	// delete/terminal access, never serialized (status lookup by ID stays
	// unauthenticated, and shouldn't leak who owns a session).
	OwnerID string `json:"-"`
	// Namespace is where this session's runner pod landed (Config.Runner-
	// Namespace). Recorded here rather than re-derived from the module on every
	// operation, since Delete/cleanup only have the session ID to go on.
	Namespace string `json:"-"`
	// nestedKubeconfig caches the fetched nested kubeconfig so the terminal
	// (D1) doesn't re-exec the runner pod on every connect. Empty for a
	// reconciled session (the in-memory cache was lost on restart) - the
	// terminal path re-fetches and repopulates it then. Guarded by Manager.mu.
	nestedKubeconfig []byte
	// CreatedAt drives the hard session cap (see StartReaper) - killed
	// once it's too old regardless of activity, so a forgotten tab left
	// open and actively typed in overnight still can't run (and bill
	// compute) forever.
	CreatedAt time.Time `json:"-"`
	// LastActivity drives the idle timeout (see StartReaper) - bumped on
	// real terminal input, not just the tab being open (see proxy.go's
	// input pump). Closing the browser tab doesn't touch this by itself;
	// it's the absence of new activity that eventually reaps the session,
	// same as if the tab had just been left open and idle. An explicit
	// stop-on-tab-close call from the frontend (best effort, fetch with
	// keepalive) is layered on top purely to free the pod sooner in the
	// common case - it's a cost-saving fast path, not what correctness
	// here depends on.
	LastActivity time.Time `json:"-"`
	// Timing is this session's own copy of the module's limits, resolved once
	// at Create (see timingFor). Copied onto the session rather than looked up
	// per reaper sweep on purpose: an admin editing a lab's duration must not
	// retroactively shorten a session already running against the old promise,
	// and the countdown the student was shown has to stay true for its whole
	// life. New sessions pick up the new value.
	Timing Timing `json:"-"`
	// driverBlobs is the resolved research-driver payload (Secret data keyed by
	// filename, plus an index.json) chosen at session-create for the research
	// labs. The API layer resolves the user's selected driver ids to entitled
	// .ko bytes before Create (where it has the DB pool and the user), so this
	// package never needs a DB handle; provisionNested mounts it as a nested
	// Secret. Nil for every non-research session and any research session with no
	// drivers selected. Written once under Manager.mu before provisioning starts.
	driverBlobs map[string][]byte
}

// Manager tracks live sessions in memory. Restart-losable by design for
// now - there's no session table in Postgres yet
// and no reaper CronJob either; both are still pending. Fine for a single
// local/dev backend replica, not fine once the backend runs with >1
// replica or needs to survive a restart.
type Manager struct {
	// clientset is the interface, not the concrete *kubernetes.Clientset, so
	// tests can inject a fake (see nested_test.go / session_test.go). Prod
	// passes the real *Clientset from NewClientset, which satisfies it.
	clientset kubernetes.Interface
	// restConfig is the OUTER cluster's REST config, kept because the
	// nested-runner path fetches a session's nested kubeconfig via a pod exec
	// (remotecommand builds its executor from this, not the typed clientset -
	// see runner.go). nil in local dev (no in-cluster config), same as
	// clientset.
	restConfig *rest.Config
	cfg        Config
	// timing resolves a module's session limits (nil = always defaults). Wired
	// from the Postgres-backed store at startup; see SetTimingSource.
	timing TimingSource
	// usage meters finished sessions and quota answers the daily allowance
	// question. Both nil-safe: unset means "no metering" / "no quota", which is
	// what local dev without a database gets. See usage.go.
	usage UsageRecorder
	quota QuotaSource

	mu       sync.Mutex
	sessions map[string]*Session
	// ended holds tombstones for sessions that have been torn down, kept for
	// endedTTL so the status endpoint can tell a student WHY their lab stopped
	// (idle vs time limit vs explicit stop) instead of a bare 404 the UI can
	// only render as a generic "session ended". Keyed by session id; pruned
	// opportunistically on write. Restart-losable like sessions itself - a lost
	// tombstone just falls back to the generic message, which is acceptable.
	ended map[string]EndedSession
	// fetching guards EnsureNestedKubeconfig so a session's nested kubeconfig
	// is background-fetched at most once at a time (not once per dashboard
	// poll). Keyed by session id; entry present = a fetch is in flight.
	fetching map[string]bool
	// ai holds the AI lab-assist plumbing (per-session terminal hubs and the
	// revocable grants the /ai/* routes check). See aihub.go. Its own lock, so
	// AI traffic never contends on mu with the session lifecycle.
	ai *aiState
}

// EndedSession is a tombstone: the reason a session stopped, plus the limits it
// ran under, kept briefly after the live session is gone. Serialized by the
// status endpoint (getSession) in place of a 404 so the lab UI can name the
// real timeout - "auto-closed after 10 min of inactivity" rather than a
// featureless "this session ended". Status is always "ended"; Reason is one of
// the EndReason* constants.
type EndedSession struct {
	ID            string    `json:"id"`
	Status        string    `json:"status"` // always "ended"
	Reason        string    `json:"reason"` // EndReason{Idle,Expired,Stopped}
	EndedAt       time.Time `json:"endedAt"`
	IdleSeconds   int       `json:"idleSeconds"`
	MaxAgeSeconds int       `json:"maxAgeSeconds"`
	// OwnerID gates the status lookup exactly as a live session's does, and is
	// never serialized - a tombstone must not leak who owned the lab either.
	OwnerID string `json:"-"`
}

// endedTTL is how long a tombstone survives after a session is torn down. Long
// enough that a student who steps away and comes back to a reaped lab still
// gets told why, short enough that the map stays small on a single replica.
const endedTTL = time.Hour

func NewManager(clientset kubernetes.Interface, restConfig *rest.Config, cfg Config) *Manager {
	return &Manager{
		clientset:  clientset,
		restConfig: restConfig,
		cfg:        cfg,
		sessions:   make(map[string]*Session),
		ended:      make(map[string]EndedSession),
		fetching:   make(map[string]bool),
		ai:         newAIState(),
	}
}

// EndedSession returns the tombstone for a session that has already been torn
// down, if one is still within endedTTL. getSession falls back to this when the
// live map has no session, so a just-reaped lab reports why it ended rather than
// 404ing into a generic message. An aged-out tombstone is pruned on read.
func (m *Manager) EndedSession(id string) (EndedSession, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.ended[id]
	if !ok {
		return EndedSession{}, false
	}
	if time.Since(e.EndedAt) > endedTTL {
		delete(m.ended, id)
		return EndedSession{}, false
	}
	return e, true
}

// recordEnded stores a tombstone for a torn-down session and prunes any that
// have aged past endedTTL. Caller holds m.mu. Ownerless sessions (local dev
// without auth) get none - there's nobody to show the message to, and the
// status endpoint gates the lookup on ownership anyway.
func (m *Manager) recordEnded(sess *Session, reason string, now time.Time) {
	for id, e := range m.ended {
		if now.Sub(e.EndedAt) > endedTTL {
			delete(m.ended, id)
		}
	}
	if sess.OwnerID == "" {
		return
	}
	m.ended[sess.ID] = EndedSession{
		ID:            sess.ID,
		Status:        "ended",
		Reason:        reason,
		EndedAt:       now,
		IdleSeconds:   int(sess.Timing.Idle / time.Second),
		MaxAgeSeconds: int(sess.Timing.MaxAge / time.Second),
		OwnerID:       sess.OwnerID,
	}
}

// SetTimingSource wires the per-lab duration overrides (Admin -> Lab
// Management). Call once at startup, before Reconcile - a session adopted with
// the default timing would otherwise outlive or undercut its module's
// configured cap. Safe to leave unset: nil means every session uses
// DefaultTiming, which is what local dev without a database gets.
func (m *Manager) SetTimingSource(src TimingSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.timing = src
}

func (m *Manager) Get(id string) (*Session, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	return s, ok
}

// RunnerNamespace is the namespace holding session-runner pods. Exposed so the
// capacity summary can exclude those pods from its node-wide baseline sweep
// without hardcoding the name (it is configurable via LABS_RUNNER_NAMESPACE).
func (m *Manager) RunnerNamespace() string {
	return m.cfg.RunnerNamespace
}

// List returns a snapshot copy of every tracked session, for the admin
// topology map (internal/topology). Value copies under the lock so callers
// can't mutate live state; the unexported fields (nestedKubeconfig, times)
// come along but aren't serialized.
func (m *Manager) List() []Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, *s)
	}
	return out
}

// NestedKubeconfigCached returns a session's nested kubeconfig WITHOUT exec-ing
// the runner. The topology dashboard uses this (not fetchNestedKubeconfig) so a
// frequent poll can't trigger an exec storm - the cache is populated by
// provisioning and by the terminal path on connect (see proxy.go). Returns
// (nil,false) for an unknown id, or a session whose cache was lost to a restart
// until the terminal repopulates it.
func (m *Manager) NestedKubeconfigCached(id string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok || len(s.nestedKubeconfig) == 0 {
		return nil, false
	}
	kc := make([]byte, len(s.nestedKubeconfig))
	copy(kc, s.nestedKubeconfig)
	return kc, true
}

// EnsureNestedKubeconfig kicks off a one-time background fetch of a running
// session's kubeconfig if it isn't cached yet, so the topology view populates on
// its own - without waiting for the student to open the terminal (the only thing
// that used to cache it). Non-blocking: the next poll picks up the cached value.
// No-op for an unknown/not-yet-running session, an already-cached one, or one a
// fetch is already in flight for (so a 15s poll can't stack goroutines).
func (m *Manager) EnsureNestedKubeconfig(id string) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok || s.Status != "running" || len(s.nestedKubeconfig) > 0 || m.fetching[id] {
		m.mu.Unlock()
		return
	}
	m.fetching[id] = true
	m.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		kc, err := m.fetchNestedKubeconfig(ctx, id)
		m.mu.Lock()
		delete(m.fetching, id)
		if err == nil {
			if s, ok := m.sessions[id]; ok {
				s.nestedKubeconfig = kc
			}
		}
		m.mu.Unlock()
	}()
}

// Reconcile adopts any session-runner pods already running in the cluster that
// aren't in this (memory-only, restart-losable - see Manager's own doc
// comment) session map. Without this, a backend restart - a dev-loop
// live_update restart, a crash, a rollout - permanently orphans every
// in-flight session: StartReaper only ever looks at m.sessions, so a pod
// it doesn't know about never gets reaped by time or idle, and its owner
// loses the ability to stop it themselves too (deleteSession's ownership
// check has nothing to compare against). Call once at startup, before
// StartReaper.
//
// CreatedAt is recovered from the pod's own creation timestamp, so the
// hard age cap still applies correctly across the restart. LastActivity
// is reset to now, not backdated - real activity before the restart isn't
// recoverable, and treating a freshly-adopted session as already idle
// would reap it immediately instead of giving it one honest grace window.
func (m *Manager) Reconcile(ctx context.Context) error {
	if m.clientset == nil {
		return nil
	}
	now := time.Now()

	runners, err := m.clientset.CoreV1().Pods(m.cfg.RunnerNamespace).List(ctx, metav1.ListOptions{LabelSelector: "app=session-runner"})
	if err != nil {
		return fmt.Errorf("reconcile: list runner pods in %s: %w", m.cfg.RunnerNamespace, err)
	}
	for _, pod := range runners.Items {
		id := pod.Labels["session"]
		module := pod.Labels["module"]
		if id == "" || module == "" {
			continue
		}
		m.adopt(&Session{
			ID: id, ModuleSlug: module, Status: "running", OwnerID: pod.Labels["owner"],
			Namespace: m.cfg.RunnerNamespace, CreatedAt: pod.CreationTimestamp.Time, LastActivity: now,
			Timing: m.timingFor(module),
		})
	}
	return nil
}

// adopt records a reconstructed session if this Manager doesn't already know
// it. LastActivity is left as the caller set it (now, not backdated - see
// Reconcile's doc).
func (m *Manager) adopt(sess *Session) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.sessions[sess.ID]; !exists {
		m.sessions[sess.ID] = sess
		slog.Info("reconciled orphaned session", "session", sess.ID, "namespace", sess.Namespace)
	}
}

// Activity records real terminal input on a session - see LastActivity.
// No-ops if the session doesn't exist (already stopped/reaped); a stray
// in-flight message losing the race against cleanup isn't worth erroring
// the whole websocket pump over.
func (m *Manager) Activity(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[id]; ok {
		s.LastActivity = time.Now()
	}
}

// Session-lifetime extension for the long-running research labs. The default
// cap is deliberately short so an idle or abandoned session frees compute fast,
// but an actively-worked research session can push its hard cap out in
// ExtendStep increments up to ExtendHardCap from creation - requested by the
// researcher (or their AI, ~5 min before the deadline) rather than granted up
// front. Only research labs are extendable; a normal challenge stays on its
// short clock.
const (
	ExtendStep    = 2 * time.Hour
	ExtendHardCap = 48 * time.Hour
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrNotRunning      = errors.New("session is not running")
	ErrNotExtendable   = errors.New("this lab's session cannot be extended")
	ErrAtHardCap       = errors.New("session already at its maximum lifetime")
)

// Extend pushes a running research session's hard cap out by ExtendStep, capped
// at ExtendHardCap from CreatedAt, and slides any live AI grant's window with it
// so the AI channel does not die mid-extend. It mutates Timing.MaxAge directly -
// NOT via Timing.Clamp, which would re-cap to MaxDuration (8h) - and since the
// reaper (expiredSessions) and the API serialization (MarshalJSON) both read
// ExpiresAt() live off Timing.MaxAge, nothing else needs updating and there is
// no persistence to write (session state is in-memory by design). Returns the
// new hard-cap time, or an error if the session is gone, not running, not an
// extendable (research) lab, or already at the 48h ceiling.
//
// Extending also counts as activity: it resets the idle clock (LastActivity).
// The session dies to whichever of its two deadlines fires first, and the idle
// one (30 min for research) is separate from the hard cap. Without this reset,
// pushing the hard cap out did nothing for a researcher who was reading a KASAN
// report or watching the eBPF pane rather than typing - they extended and were
// still idle-reaped minutes later, so the button read as broken. An explicit
// extend (by the researcher or their AI) is an unambiguous "I'm still here", so
// it slides both clocks.
func (m *Manager) Extend(id string) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return time.Time{}, ErrSessionNotFound
	}
	if s.Status != "running" {
		return time.Time{}, ErrNotRunning
	}
	if mod, ok := GetModule(s.ModuleSlug); !ok || !mod.IsResearch() {
		return time.Time{}, ErrNotExtendable
	}
	ceiling := s.CreatedAt.Add(ExtendHardCap)
	if !s.ExpiresAt().Before(ceiling) {
		return time.Time{}, ErrAtHardCap
	}
	want := s.ExpiresAt().Add(ExtendStep)
	if want.After(ceiling) {
		want = ceiling
	}
	s.Timing.MaxAge = want.Sub(s.CreatedAt)
	// Reset the idle deadline too, so the extend actually keeps the session alive
	// rather than only moving the hard cap the idle reaper would beat to it.
	s.LastActivity = time.Now()
	// Slide the AI grant's window out too (no-op if there is no grant). Lock
	// order m.mu -> m.ai.mu matches deleteWithReason's, so this cannot deadlock.
	m.extendGrant(id, ExtendStep, ceiling)
	return s.ExpiresAt(), nil
}

// StartReaper kills any running session past either of its own two deadlines -
// the hard cap from CreatedAt, or the idle timeout since its last real terminal
// input - checking every interval. Runs until ctx is cancelled.
//
// The limits come from each session's own Timing (resolved at Create from the
// module's admin-configured settings), NOT from a global passed in here, so two
// concurrent sessions of different modules expire on their own schedules.
//
// Deliberately just a goroutine, not something with its own graceful shutdown -
// the whole process exits together, and a reaper cycle that gets cut mid-sweep
// just picks the same sessions back up next time (or finds them already gone),
// same "restart-losable by design for now" as the rest of this in-memory
// Manager (see its own doc comment).
func (m *Manager) StartReaper(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.reapExpired()
			}
		}
	}()
}

func (m *Manager) reapExpired() {
	m.mu.Lock()
	expired := expiredSessions(m.sessions, time.Now())
	m.mu.Unlock()

	for _, e := range expired {
		slog.Info("reaping session", "session", e.ID, "reason", e.Reason)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := m.deleteWithReason(ctx, e.ID, e.Reason); err != nil {
			slog.Error("reap: delete failed", "session", e.ID, "error", err)
		}
		cancel()
	}
}

// expiredSession is a session past one of its deadlines, and which one. The
// reason is recorded with the usage row: a population reaped mostly on idle
// versus mostly on the hard cap says very different things about whether the
// TTLs are set right, and that is only visible if it is captured here.
type expiredSession struct {
	ID     string
	Reason string
}

// expiredSessions is the pure selection logic behind reapExpired, split out
// so it's testable without a k8s clientset (Delete needs one, this doesn't).
// Each session is judged against its OWN Timing. Caller holds m.mu.
func expiredSessions(sessions map[string]*Session, now time.Time) []expiredSession {
	var expired []expiredSession
	for id, sess := range sessions {
		if sess.Status != "running" {
			continue
		}
		// The hard cap is checked first so a session that has hit both reports the
		// promise that ran out, not the incidental fact that the student also
		// stopped typing near the end.
		switch {
		case now.After(sess.ExpiresAt()):
			expired = append(expired, expiredSession{ID: id, Reason: EndReasonExpired})
		case now.After(sess.IdleDeadline()):
			expired = append(expired, expiredSession{ID: id, Reason: EndReasonIdle})
		}
	}
	return expired
}

// BasePath is where this session's terminal is reachable through the backend's
// own reverse proxy.
func BasePath(id string) string {
	return fmt.Sprintf("/api/v1/sessions/%s/terminal", id)
}

// Create provisions a session: a disposable nested k3s cluster in its own
// microVM, into which the module's manifests (the attack target) are applied.
// See createNested and docs/architecture/nested-lab-rewrite.md.
// driverBlobs is the resolved research-driver Secret payload (see Session.
// driverBlobs); nil for non-research sessions or when no drivers were selected.
func (m *Manager) Create(ctx context.Context, moduleSlug, ownerID string, driverBlobs map[string][]byte) (*Session, error) {
	if m.clientset == nil {
		return nil, fmt.Errorf("session provisioning unavailable: not running in-cluster (docker-compose/local dev has no client-go config)")
	}

	module, ok := GetModule(moduleSlug)
	if !ok {
		return nil, fmt.Errorf("unknown module %q", moduleSlug)
	}
	// Every module is nested-shape since the legacy pod path was removed (see
	// docs/architecture/lab-session-rewrite.md, F1). A module without a Nested
	// spec is a registry bug, not a supported alternative shape.
	if module.Nested == nil {
		return nil, fmt.Errorf("module %q has no nested spec", moduleSlug)
	}

	id, err := randomID()
	if err != nil {
		return nil, err
	}
	return m.createNested(ctx, id, ownerID, module, time.Now(), driverBlobs)
}

// createNested provisions a session: one session-runner pod that boots a
// throwaway k3s cluster in its own microVM, into which the module's manifests
// (the attack target) are applied. The whole disposable cluster lives and dies
// with that single pod, so teardown (Delete) is just deleting it.
func (m *Manager) createNested(ctx context.Context, id, ownerID string, module Module, now time.Time, driverBlobs map[string][]byte) (*Session, error) {
	// Built before the lock: timingFor reads its own source and taking m.mu
	// around it would invite a deadlock for no benefit.
	sess := &Session{
		ID: id, ModuleSlug: module.Slug, Status: "starting", OwnerID: ownerID,
		Namespace: m.cfg.RunnerNamespace, CreatedAt: now, LastActivity: now,
		Timing: m.timingFor(module.Slug), driverBlobs: driverBlobs,
	}
	// Admit and reserve under ONE hold of the lock. Splitting them would let two
	// simultaneous starts both observe the old count, both pass the per-user and
	// capacity checks, and both proceed - the limits would fail precisely under
	// the concurrency they exist to handle.
	m.mu.Lock()
	if err := m.admitLocked(module, ownerID); err != nil {
		m.mu.Unlock()
		return nil, err
	}
	m.sessions[id] = sess
	m.mu.Unlock()

	// Provision in the background and return "starting" immediately. Booting a
	// whole k3s, then pulling the module's images into a fresh cluster over the
	// double-nested network, is well past a reasonable HTTP write timeout. So we
	// don't block the request: the frontend polls GET /sessions/{id} until
	// "running" (or "error"), showing a provisioning state meanwhile. The
	// goroutine uses its own context, not the request's (which is gone the moment
	// Create returns).
	go m.provisionNested(id, ownerID, module)
	return sess, nil
}

// provisionNested does the slow session build in the background (see
// createNested), flipping the session to "running" on success or "error" on
// failure (best-effort cleaning up the runner pod). The generous timeout covers
// a cold image pull into a fresh nested cluster; a hang can't outlive it and
// leak a runner.
func (m *Manager) provisionNested(id, ownerID string, module Module) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	sess, ok := m.Get(id)
	if !ok {
		return // deleted before provisioning started
	}
	fail := func(err error) {
		slog.Error("nested provision failed", "session", id, "error", err)
		m.mu.Lock()
		sess.Status, sess.Error = "error", err.Error()
		m.mu.Unlock()
		m.cleanupRunner(id)
	}

	// Phase timing. Click-to-running is a chain of strictly sequential gates
	// (pod ready -> kubeconfig fetched -> nested node ready -> manifests applied
	// -> foothold ready). Which gate dominates - and whether a change actually
	// moved it - is invisible without timing each one. Logged as structured
	// fields keyed by phase so a run of N sessions aggregates cleanly (see
	// scripts/measure-provision.sh). Pure observation: no control flow depends
	// on it, so it cannot change how a session provisions.
	start := time.Now()
	last := start
	phase := func(name string) {
		now := time.Now()
		slog.Info("provision phase", "session", id, "module", module.Slug,
			"phase", name, "ms", now.Sub(last).Milliseconds(),
			"sinceStartMs", now.Sub(start).Milliseconds())
		last = now
		// Publish the milestone so a polling client can drive a real progress bar.
		// Pure observation like the slog above: nothing here changes how the
		// session provisions. The session may have been deleted mid-provision, in
		// which case sess is a detached pointer and this write is harmless.
		m.mu.Lock()
		sess.Phase = name
		m.mu.Unlock()
	}

	if _, err := m.clientset.CoreV1().Pods(m.cfg.RunnerNamespace).Create(ctx, m.buildRunnerPod(id, ownerID, module), metav1.CreateOptions{}); err != nil {
		fail(fmt.Errorf("create runner pod: %w", err))
		return
	}
	phase("pod-created")
	if err := m.waitForReady(ctx, runnerName(id), m.cfg.RunnerNamespace); err != nil {
		fail(fmt.Errorf("runner pod not ready: %w", err))
		return
	}
	phase("pod-ready")
	kubeconfig, err := m.fetchNestedKubeconfig(ctx, id)
	if err != nil {
		fail(err)
		return
	}
	phase("kubeconfig-fetched")
	// Cache it so the terminal (D1) can exec into the nested foothold without
	// re-fetching from the runner pod on every connect.
	m.mu.Lock()
	sess.nestedKubeconfig = kubeconfig
	m.mu.Unlock()
	nestedCfg, err := nestedRESTConfig(kubeconfig)
	if err != nil {
		fail(err)
		return
	}
	if err := waitForNestedReady(ctx, nestedCfg); err != nil {
		fail(err)
		return
	}
	phase("nested-node-ready")
	// Research labs may carry user-selected drivers. Write them as a nested
	// Secret BEFORE the manifest bundle creates the workstation, so its optional
	// Secret volume (see driver-research.yaml) resolves at pod-create. Safe here
	// because sess.driverBlobs is set once before this goroutine started and
	// never mutated. Non-research/no-driver sessions skip this entirely.
	if len(sess.driverBlobs) > 0 {
		if err := createNestedDriverSecret(ctx, nestedCfg, module.Nested.EntryNamespace, sess.driverBlobs); err != nil {
			fail(fmt.Errorf("inject research drivers: %w", err))
			return
		}
	}
	if err := applyNestedManifests(ctx, nestedCfg, module.Nested.Manifests); err != nil {
		fail(fmt.Errorf("apply module manifests: %w", err))
		return
	}
	phase("manifests-applied")
	// Don't report "running" until the foothold the terminal execs into is
	// actually up - otherwise the first terminal connect races its image pull
	// and gets an empty shell.
	if module.Nested.EntryPod != "" {
		if err := waitForNestedPodReady(ctx, nestedCfg, module.Nested.EntryNamespace, module.Nested.EntryPod); err != nil {
			fail(err)
			return
		}
	}
	phase("foothold-ready")

	m.mu.Lock()
	sess.Status = "running"
	m.mu.Unlock()
	slog.Info("nested session running", "session", id, "module", module.Slug,
		"totalMs", time.Since(start).Milliseconds())
}

// cleanupRunner best-effort deletes a session's runner pod after a failed
// provision (its own short context - the request's may be spent). One pod
// deletion collapses the entire nested cluster, so there's nothing else to
// chase.
func (m *Manager) cleanupRunner(id string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := m.deleteRunner(ctx, id); err != nil {
		slog.Error("cleanup runner failed", "session", id, "error", err)
	}
}

// deleteRunner deletes a session's runner pod. NotFound is success (already
// gone). This is the whole teardown - the microVM and everything the student did
// inside its throwaway cluster go with the pod.
func (m *Manager) deleteRunner(ctx context.Context, id string) error {
	err := m.clientset.CoreV1().Pods(m.cfg.RunnerNamespace).Delete(ctx, runnerName(id), metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete runner pod: %w", err)
	}
	return nil
}

func (m *Manager) Delete(ctx context.Context, id string) error {
	return m.deleteWithReason(ctx, id, EndReasonStopped)
}

// deleteWithReason tears a session down and meters what it cost. The reason
// rides along because only the caller knows it - an explicit stop and an idle
// reap look identical by the time the pod is being deleted, and telling them
// apart afterwards is the whole point of recording it (see UsageRecord).
func (m *Manager) deleteWithReason(ctx context.Context, id, reason string) error {
	sess, ok := m.Get(id)
	if !ok {
		return nil // already gone
	}
	if err := m.deleteRunner(ctx, id); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.sessions, id)
	// Leave a tombstone in the same critical section the session is removed in,
	// so a status poll landing between the two can't see "no session, no reason".
	m.recordEnded(sess, reason, time.Now())
	m.mu.Unlock()
	// Kill any AI grant and terminal hub the moment the lab is gone - "lab closed"
	// is one of the three revocation triggers, and it must not outlive the shell it
	// pointed at. (The /ai/* routes also re-check the live session, so this is
	// belt-and-suspenders, not the only guard.)
	m.RevokeAI(id)
	m.dropHub(id)
	// After the session is out of the map, so the reservation it held is already
	// released and a queued student can be admitted without waiting on a database
	// write. Metering must never sit between a teardown and the capacity it frees.
	countEnded(reason)
	m.recordUsage(sess, reason)
	return nil
}

// labsTolerations lets a session's runner pod schedule onto the tainted labs
// node. In prod the bare-metal labs node is tainted
// (k8labs.io/dedicated=labs:NoSchedule, see infra/cluster/kubeadm-2node.md) so
// platform pods stay off the hostile machine; runner pods must tolerate that or
// they wouldn't schedule there either. Harmless where the node isn't tainted
// (local k3d): a toleration for an absent taint is a no-op, and scheduling still
// rides the nodeSelector.
func labsTolerations() []corev1.Toleration {
	return []corev1.Toleration{{
		Key:      "k8labs.io/dedicated",
		Operator: corev1.TolerationOpEqual,
		Value:    "labs",
		Effect:   corev1.TaintEffectNoSchedule,
	}}
}

// waitForReady polls a pod by name until it reports Ready.
func (m *Manager) waitForReady(ctx context.Context, name, ns string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	for {
		pod, err := m.clientset.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.Status.Phase == corev1.PodFailed {
			return fmt.Errorf("pod failed: %s", pod.Status.Reason)
		}
		for _, c := range pod.Status.Conditions {
			if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for session pod to become ready: %w", ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// randomID generates a session ID. 16 bytes (32 hex chars) because the ID is
// capability-shaped - it appears in URLs (/api/v1/sessions/{id}) and as a pod
// label - so it should not be guessable even though every session route is
// auth+ownership gated.
func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func boolPtr(b bool) *bool { return &b }
