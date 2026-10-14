// Package api registers HTTP routes. stdlib net/http's Go 1.22+ ServeMux is
// enough here - see docs/architecture/tech-stack.md for why no framework.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	httpSwagger "github.com/swaggo/http-swagger/v2"

	"github.com/jackc/pgx/v5/pgxpool"

	"backend/internal/accounts"
	"backend/internal/buildinfo"
	"backend/internal/drivers"
	"backend/internal/health"
	"backend/internal/labs"
	"backend/internal/topology"
)

func NewRouter(pool *pgxpool.Pool, labsMgr *labs.Manager, accountsCfg accounts.Config, healthChecker *health.Checker, topoBuilder *topology.Builder, timingStore *accounts.TimingStore) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", healthz(pool))
	// Unauthenticated, like /healthz: it exposes counts and byte totals, never
	// user identities or session ids. In prod the admin Traefik middleware
	// IP-allowlists this path along with the rest of the operator surface.
	mux.HandleFunc("GET /metrics", metricsHandler(labsMgr))

	// Auth: Google/GitHub sign-in (authorization code flow - logic ported
	// from our earlier Rust implementation). Cookie-based session, not
	// tokens-in-localStorage - see internal/api/auth.go.
	mux.HandleFunc("POST /api/v1/auth/google/callback", googleCallback(accountsCfg, pool))
	mux.HandleFunc("POST /api/v1/auth/github/callback", githubCallback(accountsCfg, pool))
	mux.HandleFunc("GET /api/v1/auth/me", requireAuth(pool, me(pool)))
	mux.HandleFunc("POST /api/v1/auth/logout", logout)
	// Localhost-only username/password login. Mounted ONLY when DEV_LOGIN=1
	// (docker-compose sets it; prod never does), so this route is absent in a
	// real deployment. See internal/api/devlogin.go for the full rationale.
	if devLoginEnabled() {
		mux.HandleFunc("POST /api/v1/auth/dev/login", devLogin(pool))
		slog.Warn("DEV_LOGIN enabled: mounting localhost-only username/password auth - never enable this in prod")
	}

	// Leaderboard. Intentionally public: the top ranks are shown on the landing
	// page to signed-out visitors, so unlike the rest of /api/v1 this must be
	// reachable without a session. It discloses only the leaders' usernames and
	// lab stats, never emails or session data, and gates the full board (and the
	// operator count) behind a signed-in caller who has cleared a lab - resolved
	// per-request via optionalUser, not middleware. See leaderboard.go.
	mux.HandleFunc("GET /api/v1/leaderboard", leaderboard(pool))

	// Public operator profiles. Same public disclosure as the leaderboard -
	// handle, real stats, self-published bio/website, and the activity calendar,
	// never an email or session detail. /operators is the landing showcase (a
	// collection route, so it can't be shadowed by a handle); /operators/{username}
	// is one profile, and its avatar has its own public route because a signed-out
	// visitor must be able to render it (the /users/{id}/avatar route stays
	// auth-gated for the admin list).
	mux.HandleFunc("GET /api/v1/operators", operatorShowcase(pool))
	mux.HandleFunc("GET /api/v1/operators/{username}", operatorProfile(pool))
	mux.HandleFunc("GET /api/v1/operators/{username}/avatar", serveOperatorAvatar(pool))

	// Self-service profile (Settings -> General). Everything here acts on the
	// caller's own account - there's no user ID in the path, it comes from the
	// session - so a user can't edit anyone else's profile.
	mux.HandleFunc("GET /api/v1/profile", requireAuth(pool, getProfile(pool)))
	mux.HandleFunc("PATCH /api/v1/profile", requireAuth(pool, updateProfile(pool)))
	// Display name (the username handle) is on its own route rather than folded
	// into PATCH /profile: /profile is autosaved on every keystroke, but claiming
	// a UNIQUE public handle wants a deliberate save, and it changes the caller's
	// /operators/{username} URL. See accounts.UpdateUsername.
	mux.HandleFunc("PATCH /api/v1/profile/username", requireAuth(pool, updateUsername(pool)))
	mux.HandleFunc("POST /api/v1/profile/avatar", requireAuth(pool, uploadAvatar(pool)))
	mux.HandleFunc("DELETE /api/v1/profile/avatar", requireAuth(pool, deleteAvatar(pool)))
	// Reading an avatar takes an ID because avatars appear in the admin Users
	// list, not just on your own settings page. Auth-gated but not self-scoped;
	// it serves a picture the user chose to publish.
	mux.HandleFunc("GET /api/v1/users/{id}/avatar", requireAuth(pool, serveAvatar(pool)))

	// API keys. A key authenticates as its owner and carries exactly their
	// permissions - see accounts/apikeys.go. Managing keys is deliberately
	// session-only in practice: these routes work with a key too, but a key
	// can only ever list and revoke its own owner's keys.
	mux.HandleFunc("GET /api/v1/apikeys", requireAuth(pool, listAPIKeys(pool)))
	mux.HandleFunc("POST /api/v1/apikeys", requireAuth(pool, createAPIKey(pool)))
	mux.HandleFunc("DELETE /api/v1/apikeys/{id}", requireAuth(pool, deleteAPIKey(pool)))

	// Catalog: the labs this caller may actually run. No longer public - it
	// used to be, on the theory that browsing the catalog is harmless, but
	// access is now grant-based (registration is open, entitlement is not), so
	// an unauthenticated caller has no entitlements to compute a list from.
	// Admins see the whole catalog; a user sees only granted modules, which is
	// an empty list until an admin grants one.
	mux.HandleFunc("GET /api/v1/modules", requireAuth(pool, listModules(pool)))

	// A lab's guided course (internal/course). Entitlement-gated on the same
	// grant as starting the lab - the lesson names the flaw the lab teaches, so
	// handing it to someone who can't run the module would leak the answer.
	mux.HandleFunc("GET /api/v1/modules/{slug}/course", requireAuth(pool, getCourse(pool)))

	// Course progress. Always scoped to the caller (the user id comes from the
	// session, never the path). Completion is the captured CTF flag, checked
	// server-side against the module's expected flag (POST .../flag) - a client
	// cannot declare itself complete. Quizzes (PUT) are formative and don't gate
	// completion; DELETE clears the row to restart the lab. See progress.go.
	mux.HandleFunc("GET /api/v1/modules/{slug}/progress", requireAuth(pool, getProgress(pool)))
	mux.HandleFunc("PUT /api/v1/modules/{slug}/progress", requireAuth(pool, putAnswer(pool)))
	mux.HandleFunc("DELETE /api/v1/modules/{slug}/progress", requireAuth(pool, deleteProgress(pool)))
	mux.HandleFunc("POST /api/v1/modules/{slug}/flag", requireAuth(pool, putFlag(pool)))

	// Research driver catalog (see internal/api/drivers.go). Both are gated inside
	// the handler on holding a research-lab grant, not just requireAuth - the
	// catalog is a research-only surface. Admin vetting is under /admin below.
	mux.HandleFunc("GET /api/v1/drivers", requireAuth(pool, listDrivers(pool)))
	mux.HandleFunc("POST /api/v1/drivers", requireAuth(pool, uploadDriver(pool)))

	// Session lifecycle: provisions a tenant microVM (target app + EDR
	// sensor) for the chosen module. Auto-stops on its own too - see
	// internal/labs/session.go's StartReaper (60min hard cap, 3min idle
	// timeout off real terminal input) - explicit DELETE here is the fast
	// path, not the only path. Every route here requires auth *and*
	// ownership (checked against Session.OwnerID, set at Create time from
	// the caller's userID) - including the status read. It was previously
	// unauthenticated on the theory that a random session ID is itself the
	// capability, but randomID() was only 4 bytes at the time (since widened
	// to 16, see internal/labs/session.go) and the sole caller - LabPage's
	// reconnect poll - already sends credentials, so the exemption bought
	// nothing. The ID is not the access control; the gate here is.
	mux.HandleFunc("POST /api/v1/sessions", requireAuth(pool, createSession(pool, labsMgr)))
	mux.HandleFunc("GET /api/v1/sessions/{id}", requireAuth(pool, getSession(labsMgr)))
	mux.HandleFunc("DELETE /api/v1/sessions/{id}", requireAuth(pool, deleteSession(labsMgr)))
	mux.HandleFunc("POST /api/v1/sessions/{id}/extend", requireAuth(pool, extendSession(labsMgr)))
	mux.HandleFunc("POST /api/v1/sessions/{id}/keepalive", requireAuth(pool, keepaliveSession(labsMgr)))

	// Subtree match ("/" suffix) - proxies ttyd's own HTTP assets and its
	// websocket upgrade alike. See internal/labs/proxy.go. Auth + ownership
	// gated the same as delete - this is an interactive read/write shell
	// into the session's pod, not just status.
	mux.HandleFunc("/api/v1/sessions/{id}/terminal/", requireAuth(pool, terminalProxy(labsMgr)))

	// The student-facing topology of their OWN lab cluster. Ownership-gated the
	// same as the terminal, and built from a separate source rather than by
	// filtering the admin map - see internal/topology/scoped.go for why that
	// distinction is the security design and not a style choice.
	mux.HandleFunc("GET /api/v1/sessions/{id}/topology", requireAuth(pool, sessionTopology(labsMgr)))

	// The Defender pane: a read-only websocket streaming the in-guest eBPF
	// sensor's event feed for eBPF modules (Module.EBPF). Ownership-gated exactly
	// like the terminal - it's a live view into the student's own session - and a
	// 404 for a module with no sensor. See internal/labs/defender.go.
	mux.HandleFunc("/api/v1/sessions/{id}/defender", requireAuth(pool, defenderStream(labsMgr)))

	// Live telemetry for the Investigation UI - Server-Sent Events, not
	// WebSocket (see tech-stack.md). Still pending.
	mux.HandleFunc("GET /api/v1/sessions/{id}/events", notImplemented)

	// AI lab-assist. The owner mints/revokes a short-lived, session-scoped grant
	// for an external AI (ownership-gated exactly like the terminal). The AI then
	// uses that grant on the /ai/* routes ONLY - a ScopeAI token is refused by
	// requireAuth everywhere else, and there is deliberately no flag or
	// lifecycle route here, so the AI can read the lesson/topology/terminal and
	// run commands (mirrored on the student's own terminal) but can never submit
	// a flag, stop the lab, or reach another session. See internal/api/ai.go.
	mux.HandleFunc("POST /api/v1/sessions/{id}/ai-grant", requireAuth(pool, aiGrantCreate(labsMgr)))
	mux.HandleFunc("DELETE /api/v1/sessions/{id}/ai-grant", requireAuth(pool, aiGrantRevoke(labsMgr)))
	mux.HandleFunc("PUT /api/v1/sessions/{id}/active-tab", requireAuth(pool, aiSetActiveTab(labsMgr)))
	mux.HandleFunc("PUT /api/v1/sessions/{id}/guide-step", requireAuth(pool, aiSetGuideStep(labsMgr)))
	mux.HandleFunc("GET /api/v1/ai/sessions/{id}/context", requireAIGrant(labsMgr, aiContext(labsMgr)))
	mux.HandleFunc("POST /api/v1/ai/sessions/{id}/exec", requireAIGrant(labsMgr, aiExec(labsMgr)))
	mux.HandleFunc("POST /api/v1/ai/sessions/{id}/extend", requireAIGrant(labsMgr, aiExtend(labsMgr)))
	mux.HandleFunc("/api/v1/ai/sessions/{id}/terminal", requireAIGrant(labsMgr, aiTerminal(labsMgr)))

	// Admin-only: pod health across the cluster's own components (see
	// internal/health) and reverse-proxied links to Grafana/ArgoCD/Vault's
	// UIs (see adminproxy.go). requireAdmin, not just requireAuth - and for
	// the proxies, cookie-only in practice (a plain browser
	// navigation/iframe can't attach a custom Authorization header, same
	// structural reason the terminal proxy is cookie-only too).
	mux.HandleFunc("GET /api/v1/admin/health", requireAdmin(pool, adminHealth(healthChecker)))
	// Live topology map: nodes -> namespaces -> pods -> containers, plus the
	// static wiring (edges/layers/dashboards) and active sessions. Admin only,
	// same read-only clientset as /admin/health. See internal/topology.
	mux.HandleFunc("GET /api/v1/admin/topology", requireAdmin(pool, adminTopology(topoBuilder, labsMgr, pool)))

	// User management: list accounts with their lab entitlements, grant/revoke
	// a module, block/unblock. Revoking and blocking both tear down whatever
	// the user currently has running (see adminusers.go) - leaving a live shell
	// up after cutting someone's access would make the revoke cosmetic.
	mux.HandleFunc("GET /api/v1/admin/users", requireAdmin(pool, adminListUsers(pool, labsMgr)))
	mux.HandleFunc("POST /api/v1/admin/users/{id}/grants", requireAdmin(pool, adminGrantLab(pool)))
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}/grants/{slug}", requireAdmin(pool, adminRevokeLab(pool, labsMgr)))
	mux.HandleFunc("POST /api/v1/admin/users/{id}/blocked", requireAdmin(pool, adminSetBlocked(pool, labsMgr)))
	mux.HandleFunc("POST /api/v1/admin/users/{id}/quota", requireAdmin(pool, adminSetUserQuota(pool)))
	mux.HandleFunc("DELETE /api/v1/admin/users/{id}", requireAdmin(pool, adminDeleteUser(pool, labsMgr)))

	// Lab management: per-lab session duration + idle timeout (see adminlabs.go).
	// Changing these changes how long a student's work survives, so they're
	// admin-gated and validated rather than clamped.
	mux.HandleFunc("GET /api/v1/admin/labs/settings", requireAdmin(pool, adminListLabSettings(pool)))
	mux.HandleFunc("PUT /api/v1/admin/labs/settings/{slug}", requireAdmin(pool, adminSetLabSetting(pool, timingStore)))
	mux.HandleFunc("DELETE /api/v1/admin/labs/settings/{slug}", requireAdmin(pool, adminResetLabSetting(pool, timingStore)))

	// Research driver vetting (see internal/api/admindrivers.go).
	mux.HandleFunc("GET /api/v1/admin/drivers", requireAdmin(pool, adminListDrivers(pool)))
	mux.HandleFunc("POST /api/v1/admin/drivers/{id}/decision", requireAdmin(pool, adminDriverDecision(pool)))
	mux.HandleFunc("GET /api/v1/admin/drivers/{id}/download", requireAdmin(pool, adminDownloadDriver(pool)))

	// Grafana/ArgoCD proxies (adminproxy.go) - deliberately unannotated
	// for swaggo, same reasoning as terminalProxy: HTML/asset passthrough
	// to a third-party app, not a JSON endpoint of ours.
	mux.HandleFunc("/admin/grafana/", requireAdmin(pool, adminProxy("grafana.observability.svc.cluster.local:80", "")))
	mux.HandleFunc("/admin/argocd/", requireAdmin(pool, adminProxy("argocd-server.argocd.svc.cluster.local:80", "")))

	// Vault: NOT under /admin/vault - Vault's UI hardcodes absolute
	// /ui/... and /v1/... paths with no subpath config option (confirmed
	// against hashicorp/vault#18668, still open). Mounting it at the paths it
	// already expects, at our own domain root, sidesteps the rewrite
	// problem entirely - same requireAdmin gate as Grafana/ArgoCD, just
	// matched on the literal paths Vault's own JS requests instead of an
	// /admin/* prefix. Confirmed via live network capture this is the
	// complete set Vault's UI calls (static assets + its actual /v1 API -
	// there's no separate "UI-only" surface, this proxies Vault's full API).
	mux.HandleFunc("/ui/", requireAdmin(pool, adminProxy("vault.vault.svc.cluster.local:8200", "")))
	mux.HandleFunc("/v1/", requireAdmin(pool, adminProxy("vault.vault.svc.cluster.local:8200", "")))

	// Swagger UI - admin-gated like the other reverse-proxied panels
	// (Grafana/ArgoCD/Vault): requireAdmin here + IP-allowlisted at the ingress
	// (backend chart's ingress.adminPaths includes /swagger). Served at
	// /swagger/ so it matches the dashboard link (topology.Dashboards) and sits
	// on its own gated path instead of under the public /api. Spec is generated
	// by `make swagger` and registered via cmd/server/main.go's blank import of
	// the docs package.
	// Swagger UI - now requireAuth, not requireAdmin. Every signed-in user gets
	// API docs, but the spec behind it is filtered to their access level (see
	// apidocs.go): a user sees auth/profile/api-key/module/session operations,
	// an admin additionally sees every /admin/* route. Serving the full spec to
	// everyone would list a dozen operations a user can only ever get 403 from,
	// and hand anyone who compromised a low-privilege account a map of the
	// admin surface.
	//
	// doc.json is registered before the UI so the more specific pattern wins;
	// it's what the UI fetches to render.
	mux.HandleFunc("GET /swagger/doc.json", requireAuth(pool, scopedSwaggerSpec()))
	mux.HandleFunc("/swagger/", requireAuth(pool, httpSwagger.Handler(httpSwagger.URL("/swagger/doc.json")).ServeHTTP))

	return mux
}

// healthz is a k8s liveness/readiness probe target, not part of the API
// surface API consumers care about - deliberately outside @BasePath and
// left unannotated for swaggo rather than fighting its per-route
// basePath limitations for an endpoint nobody browsing the docs needs.
func healthz(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "db unreachable"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "sha": buildinfo.SHA})
	}
}

// listModules godoc
// @Summary     List lab modules
// @Description The lab modules the caller is entitled to. Admins get the full catalog; a regular user gets the free first lab plus everything progression has unlocked (each tier opens when the previous is solved) and any explicit grants. The X-Modules-Locked response header reports how many catalog labs are still locked for the caller (0 for admins).
// @Tags        labs
// @Produce     json
// @Success     200 {array} labs.Module
// @Failure     401 {string} string "unauthorized"
// @Security    BearerAuth
// @Router      /modules [get]
func listModules(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := contextUser(r)
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		all := labs.ListModules()

		// Admins bypass entitlement and progression and see the whole catalog;
		// regular users see only what they're entitled to - explicit grants folded
		// in with the progression unlocks (the free tier-0 lab, plus each tier the
		// previous one unlocked; see accounts.EntitledModules). Only non-admins run
		// the grants query, so an admin's catalog still never depends on it.
		var entitled []string
		if !accounts.IsAdmin(u.Role) {
			var err error
			entitled, err = accounts.EntitledModules(r.Context(), pool, u.ID)
			if err != nil {
				http.Error(w, "failed to read entitlements", http.StatusInternalServerError)
				return
			}
		}
		visible := visibleModules(all, accounts.IsAdmin(u.Role), entitled)

		// Decorate each visible module with whether the caller has solved it, so
		// the sidebar can show progression state (solved vs current vs available)
		// and a "N cleared" count without a second round trip. Admins get this too:
		// they run the missions, not only grant them, so a solved lab shows its
		// green check for every account - the earlier admin path skipped this
		// decoration, which is why solved labs never turned green for admins.
		solved, err := accounts.SolvedModules(r.Context(), pool, u.ID)
		if err != nil {
			http.Error(w, "failed to read progress", http.StatusInternalServerError)
			return
		}
		views := make([]moduleView, 0, len(visible))
		for _, m := range visible {
			views = append(views, moduleView{Module: m, Completed: solved[m.Slug]})
		}
		// How many catalog labs the caller still can't see - i.e. labs left to
		// unlock by progressing. The sidebar's "Encrypted" teaser keys off this so
		// it disappears once a user has unlocked the whole catalog, rather than
		// claiming "the rest is locked" when nothing is. Always 0 for admins, who
		// see everything. A header keeps the array body unchanged for every other
		// caller of this endpoint.
		w.Header().Set("X-Modules-Locked", strconv.Itoa(len(all)-len(visible)))
		json.NewEncoder(w).Encode(views)
	}
}

// moduleView is a catalog module plus the calling user's completion of it. Kept
// at the API boundary (not on labs.Module) because "completed" is per-caller
// state, not a property of the catalog. Embeds Module so its JSON shape is
// unchanged apart from the added `completed` field.
type moduleView struct {
	labs.Module
	Completed bool `json:"completed"`
}

// filterModules keeps only the modules whose slug appears in granted. Split out
// from listModules so the entitlement filtering can be tested without a
// database - the surrounding handler needs a pool, this doesn't.
//
// Returns a non-nil slice so a user with no grants encodes as [] rather than
// null, which the frontend maps over directly.
func filterModules(all []labs.Module, granted []string) []labs.Module {
	allowed := make(map[string]bool, len(granted))
	for _, slug := range granted {
		allowed[slug] = true
	}
	visible := []labs.Module{}
	for _, m := range all {
		if allowed[m.Slug] {
			visible = append(visible, m)
		}
	}
	return visible
}

// visibleModules picks the modules a caller may see: the whole catalog for an
// admin, otherwise only those the entitlement set grants. Pure (no DB) so the
// admin-vs-user selection is unit-testable; the completion decoration that the
// handler layers on top is what needs the pool.
func visibleModules(all []labs.Module, isAdmin bool, entitled []string) []labs.Module {
	if isAdmin {
		return all
	}
	return filterModules(all, entitled)
}

// createSession godoc
// @Summary     Start a lab session
// @Description Provisions a per-session pod (target app + EDR sensor) for the chosen module. Ownership is recorded from the caller's token.
// @Tags        labs
// @Accept      json
// @Produce     json
// @Param       body body object true "{\"module\": \"suid-privesc-101\"}"
// @Success     200 {object} labs.Session
// @Failure     403 {string} string "forbidden - no grant for this module"
// @Failure     502 {object} labs.Session "session provisioning failed - Session.Status is \"error\", Session.Error has the reason"
// @Security    BearerAuth
// @Router      /sessions [post]
func createSession(pool *pgxpool.Pool, mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Module string `json:"module"`
			// Drivers is an optional set of research-driver ids to mount into the
			// session (research labs only). Entitlement is re-checked when they are
			// resolved to bytes, so a caller cannot mount a module it may not use.
			Drivers []string `json:"drivers"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		u := contextUser(r)
		if u == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		// Entitlement is enforced here as well as in listModules: hiding a
		// module from the catalog is presentation, not access control - the
		// caller picks the slug, so this is the check that actually holds.
		ok, err := accounts.HasGrant(r.Context(), pool, u.ID, u.Role, body.Module)
		if err != nil {
			http.Error(w, "failed to read entitlements", http.StatusInternalServerError)
			return
		}
		if !ok {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		// Resolve any selected drivers to their entitled .ko bytes, but only for
		// the research labs that can mount them - ResolveForSession itself filters
		// to approved drivers this user may use, so an id for a pending, denied, or
		// someone else's private module is silently dropped rather than mounted.
		var driverBlobs map[string][]byte
		if mod, ok := labs.GetModule(body.Module); ok && mod.IsResearch() && len(body.Drivers) > 0 {
			driverBlobs, err = drivers.ResolveForSession(r.Context(), pool, u.ID, body.Drivers)
			if err != nil {
				http.Error(w, "failed to resolve drivers", http.StatusInternalServerError)
				return
			}
		}

		userID := u.ID
		sess, err := mgr.Create(r.Context(), body.Module, userID, driverBlobs)
		if ae, ok := labs.IsAdmission(err); ok {
			// A limit said no. This is not a provisioning failure and must not be
			// dressed as one: the student can act on it, and the frontend should
			// show the sentence rather than a generic "session error".
			//
			// The two reasons get different codes because they mean different
			// things to a client and to a proxy. Hitting your own concurrency cap
			// is 409 - the conflict is with your own running lab, and retrying
			// unchanged will never help. A full platform is 503 with Retry-After,
			// which is the standard "not you, come back" and is safe to retry.
			status := http.StatusConflict
			switch ae.Reason {
			case labs.ReasonNodeCapacity:
				status = http.StatusServiceUnavailable
				if ae.RetryAfter > 0 {
					w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(ae.RetryAfter.Seconds()))))
				}
			case labs.ReasonDailyQuota:
				// 429: the user has spent an allowance that refills with time.
				// Distinct from 409 (fix it yourself right now, by stopping a lab)
				// and from 503 (nothing to do with you).
				status = http.StatusTooManyRequests
			}
			resp := map[string]string{
				"error":  ae.Message,
				"reason": ae.Reason,
			}
			// For a "you already have a lab running" refusal, tell the client which
			// one, so it can offer to stop that lab and start this one in one click
			// rather than dead-ending the student on the error sentence.
			if ae.ConflictSessionID != "" {
				resp["conflictSessionId"] = ae.ConflictSessionID
				resp["conflictModule"] = ae.ConflictModule
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(resp)
			return
		}
		if err != nil {
			// Create returns a nil *Session (not one with Status: "error")
			// for early failures - unknown module, or no in-cluster config
			// at all (docker-compose/local dev, see internal/labs.Manager).
			// Encoding nil as the response body would land as a bare `null`,
			// which the frontend can't do anything with - build a proper
			// error-shaped body instead.
			if sess == nil {
				sess = &labs.Session{ModuleSlug: body.Module, Status: "error", Error: err.Error()}
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(sess)
			return
		}
		// Stamp the solve clock at lab-session start, so fastest-solve is measured
		// from when the student actually began - not from a progress row created at
		// flag-submission time, which made started_at ~ completed_at and produced
		// bogus ~1s ("0:01") solves. Reset started_at on a re-start of an unsolved
		// lab; never disturb an already-completed row (its solve_seconds is frozen).
		// Best-effort: a failure here must not fail the session the student just got.
		if _, err := pool.Exec(r.Context(),
			`INSERT INTO lab_progress (user_id, module_slug, started_at)
			 VALUES ($1, $2, now())
			 ON CONFLICT (user_id, module_slug) DO UPDATE
			   SET started_at = now(), updated_at = now()
			   WHERE lab_progress.completed_at IS NULL`, userID, body.Module); err != nil {
			slog.Warn("createSession: stamp solve start", "user", userID, "module", body.Module, "error", err)
		}
		json.NewEncoder(w).Encode(sess)
	}
}

// getSession godoc
// @Summary     Get session status
// @Description Requires auth *and* ownership (Session.OwnerID must match the caller) - the same gate as delete/terminal.
// @Tags        labs
// @Produce     json
// @Param       id path string true "Session ID"
// @Success     200 {object} labs.Session
// @Failure     403 {string} string "forbidden - not this session's owner"
// @Failure     404 {string} string "not found"
// @Security    BearerAuth
// @Router      /sessions/{id} [get]
func getSession(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		userID, _ := r.Context().Value(userIDKey).(string)
		if sess, ok := mgr.Get(id); ok {
			if sess.OwnerID != userID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			json.NewEncoder(w).Encode(sess)
			return
		}
		// No live session - but it may have been reaped seconds ago. A tombstone
		// (kept for endedTTL) lets the UI say WHY the lab stopped (idle vs time
		// limit) instead of a bare 404 it can only render as "session ended".
		// Same ownership gate as a live session, so it 403s (not 404s) for a
		// non-owner, matching the live path and not leaking existence differently.
		if ended, ok := mgr.EndedSession(id); ok {
			if ended.OwnerID != userID {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			json.NewEncoder(w).Encode(ended)
			return
		}
		http.NotFound(w, r)
	}
}

// deleteSession godoc
// @Summary     Stop a lab session
// @Description Requires auth *and* ownership (Session.OwnerID must match the caller) - any other authenticated user gets 403, not just 401.
// @Tags        labs
// @Param       id path string true "Session ID"
// @Success     204 "no content"
// @Failure     403 {string} string "forbidden - not this session's owner"
// @Failure     404 {string} string "not found"
// @Security    BearerAuth
// @Router      /sessions/{id} [delete]
func deleteSession(mgr *labs.Manager) http.HandlerFunc {
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
		if err := mgr.Delete(r.Context(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// extendSession godoc
// @Summary     Extend a research lab session
// @Description Pushes a running research session's hard cap out by 2h (up to a 48h cap), sliding any live AI grant with it. Requires auth *and* ownership; only research labs are extendable. Returns the updated session.
// @Tags        labs
// @Param       id path string true "Session ID"
// @Success     200 {object} labs.Session
// @Failure     403 {string} string "forbidden - not the owner, or lab is not extendable"
// @Failure     404 {string} string "not found"
// @Failure     409 {string} string "not running, or already at the 48h cap"
// @Security    BearerAuth
// @Router      /sessions/{id}/extend [post]
func extendSession(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		// Ownership gate first, before Extend touches anything - a non-owner must
		// get 403 without learning whether the session is extendable or at its cap.
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
		if _, err := mgr.Extend(id); err != nil {
			switch {
			case errors.Is(err, labs.ErrSessionNotFound):
				http.NotFound(w, r)
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
		// Re-read so the UI gets the fresh expiresAt (MarshalJSON derives it live
		// off the just-bumped Timing.MaxAge).
		if updated, ok := mgr.Get(id); ok {
			json.NewEncoder(w).Encode(updated)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// keepaliveSession resets a running session's idle clock on behalf of its owner.
// It is the "I'm still here" presence signal the browser sends on an interval
// WHILE the lab tab is visible - so a researcher watching a long fuzzer or build
// (which only produces terminal OUTPUT, never input, and so never resets idle on
// its own) does not get idle-reaped out from under a job they are actively
// watching. It is deliberately gated on visibility client-side, not fired
// unconditionally: a hidden or closed tab stops pinging, so a truly abandoned
// session still reaps. The hard cap is unchanged - this only slides the idle
// deadline, never the ceiling. Ownership-gated; a no-op past the session's life.
// @Summary     Keep a running session's idle timer alive
// @Description Owner-only presence ping (sent by the browser while the lab tab is visible) that resets the idle timeout. Does not touch the hard cap.
// @Tags        sessions
// @Param       id path string true "Session ID"
// @Success     204 {string} string "idle clock reset (or session already gone)"
// @Failure     403 {string} string "forbidden - not the owner"
// @Failure     404 {string} string "not found"
// @Security    BearerAuth
// @Router      /sessions/{id}/keepalive [post]
func keepaliveSession(mgr *labs.Manager) http.HandlerFunc {
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
		// Only a live session has an idle clock worth resetting; a tombstone is
		// harmless but pointless to touch.
		if sess.Status == "running" {
			mgr.Activity(id)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// terminalProxy gates labs.Manager.TerminalProxy behind the same
// authentication + ownership check as deleteSession - it's a full
// interactive read/write shell into the session's pod, not a status read.
// Deliberately unannotated for swaggo - this proxies raw HTTP/websocket
// bytes to ttyd, there's no JSON request/response shape to describe.
func terminalProxy(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := mgr.Get(r.PathValue("id"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		userID, _ := r.Context().Value(userIDKey).(string)
		if sess.OwnerID != userID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		mgr.TerminalProxy(w, r)
	}
}

// defenderStream gates labs.Manager.DefenderStream behind the same auth +
// ownership check as the terminal - it's a live view into the caller's own
// session. A 404 for a module without a Defender feed keeps the gate ahead of
// the stream, same shape as terminalProxy. Unannotated for swaggo (raw
// websocket bytes, no JSON shape).
func defenderStream(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok := mgr.Get(r.PathValue("id"))
		if !ok {
			http.NotFound(w, r)
			return
		}
		userID, _ := r.Context().Value(userIDKey).(string)
		if sess.OwnerID != userID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		mgr.DefenderStream(w, r, sess)
	}
}

// adminHealth godoc
// @Summary     Cluster health
// @Description Pod status across every namespace the platform's own components use (see internal/health.Namespaces). Admin only.
// @Tags        admin
// @Produce     json
// @Success     200 {array} health.Namespace
// @Failure     403 {string} string "forbidden - not an admin"
// @Security    BearerAuth
// @Router      /admin/health [get]
func adminHealth(checker *health.Checker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		namespaces, err := checker.ClusterHealth(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(namespaces)
	}
}

// adminTopology godoc
// @Summary     Cluster topology
// @Description Live map of nodes, namespaces, pods and containers, joined with the static wiring (edges, host-substrate layers, dashboards) and active lab sessions. Admin only.
// @Tags        admin
// @Produce     json
// @Success     200 {object} topology.Topology
// @Failure     403 {string} string "forbidden - not an admin"
// @Failure     503 {string} string "topology unavailable - not running in-cluster"
// @Router      /admin/topology [get]
func adminTopology(builder *topology.Builder, labsMgr *labs.Manager, pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		topo, err := builder.Build(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		// Capacity summary: what the labs node(s) offer vs what live sessions
		// reserve, plus the admission budget. Memory allocatable is summed over
		// labs-role nodes only, since that is where sessions are scheduled - the
		// control plane's memory is not lab capacity.
		cap := &topology.Capacity{MemBudgetBytes: labsMgr.Stats().BudgetBytes}
		labsNodes := map[string]bool{}
		for _, n := range topo.Nodes {
			if n.Role == "labs" {
				cap.MemAllocatableBytes += n.AllocatableMemBytes
				cap.CPUAllocatableMillis += n.AllocatableCPUMillis
				labsNodes[n.Name] = true
			}
		}
		// The platform baseline: requests of the non-session pods sharing the labs
		// node (tetragon, CNI, coredns, ...). Sessions live in RunnerNamespace and
		// are counted as MemReserved below, so that namespace is skipped here to
		// avoid double-counting the very labs this baseline sits beneath.
		for _, ns := range topo.Namespaces {
			if ns.Name == labsMgr.RunnerNamespace() {
				continue
			}
			for _, p := range ns.Pods {
				if labsNodes[p.Node] {
					cap.MemSystemReservedBytes += p.MemRequestBytes
					cap.CPUSystemReservedMillis += p.CPURequestMillis
				}
			}
		}
		// Per-owner accumulator for the attribution breakdown, filled alongside
		// the session loop below.
		byOwner := map[string]*topology.CapacityUser{}

		// Attach active sessions and a provisioning edge per session (backend
		// -> session:<id>). For a session whose nested kubeconfig is already
		// cached, enumerate its nested cluster too - cache-only (no runner
		// exec) so a frequent dashboard poll stays cheap; see
		// Manager.NestedKubeconfigCached.
		for _, s := range labsMgr.List() {
			ts := topology.Session{ID: s.ID, Module: s.ModuleSlug, Status: s.Status}
			// Live cost of this session: reserved memory, and GiB-minutes accrued
			// since it started. Lets the admin map show what each lab is costing
			// rather than only that it is running.
			mem := labs.SessionMemoryBySlug(s.ModuleSlug)
			reserved := mem.Value()
			cpu := labs.SessionCPUMillisBySlug(s.ModuleSlug)
			ts.ReservedBytes = reserved
			cap.MemReservedBytes += reserved
			cap.CPUReservedMillis += cpu
			if o := byOwner[s.OwnerID]; o != nil {
				o.MemBytes += reserved
				o.CPUMillis += cpu
				o.Sessions++
			} else {
				byOwner[s.OwnerID] = &topology.CapacityUser{MemBytes: reserved, CPUMillis: cpu, Sessions: 1}
			}
			if mins := time.Since(s.CreatedAt).Minutes(); mins > 0 {
				ts.GiBMinutes = float64(reserved) / (1 << 30) * mins
			}
			if kc, ok := labsMgr.NestedKubeconfigCached(s.ID); ok {
				ts.Cluster = topology.NestedFromKubeconfig(r.Context(), kc)
			} else {
				// Not cached yet (e.g. a reconciled session post-restart, or the
				// terminal was never opened) - kick off a one-time background
				// fetch so the nested view fills in on a later poll.
				labsMgr.EnsureNestedKubeconfig(s.ID)
			}
			topo.Sessions = append(topo.Sessions, ts)
			// Provisioning edge from the backend to the session's runner, which
			// exposes the nested API on :6443. The frontend resolves
			// session:<id> to the runner pod (labs-runners) by id.
			topo.Edges = append(topo.Edges, topology.Edge{
				From: "backend", To: "session:" + s.ID, Port: ":6443", Kind: "prov",
			})
		}
		cap.ByUser = capacityByUser(r.Context(), pool, byOwner)
		topo.Capacity = cap
		json.NewEncoder(w).Encode(topo)
	}
}

// capacityMaxUsers caps the attribution breakdown: past this many users the
// tail folds into one "others" row, so the stacked column stays readable rather
// than becoming a stack of slivers.
const capacityMaxUsers = 6

// capacityByUser turns the per-owner accumulator into the sorted, labelled,
// capped breakdown the charts render. Heaviest memory first (that is the
// binding resource); usernames resolved in one query, with a short-ID fallback
// for an owner whose account is already gone. On a username lookup failure it
// degrades to short IDs rather than dropping the breakdown - attribution by ID
// still beats none.
func capacityByUser(ctx context.Context, pool *pgxpool.Pool, byOwner map[string]*topology.CapacityUser) []topology.CapacityUser {
	if len(byOwner) == 0 {
		return nil
	}
	ids := make([]string, 0, len(byOwner))
	for id := range byOwner {
		ids = append(ids, id)
	}
	names, err := accounts.UsernamesByIDs(ctx, pool, ids)
	if err != nil {
		names = nil
	}

	out := make([]topology.CapacityUser, 0, len(byOwner))
	for id, u := range byOwner {
		label := names[id]
		if label == "" {
			label = shortOwnerID(id)
		}
		out = append(out, topology.CapacityUser{User: label, MemBytes: u.MemBytes, CPUMillis: u.CPUMillis, Sessions: u.Sessions})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].MemBytes != out[j].MemBytes {
			return out[i].MemBytes > out[j].MemBytes
		}
		return out[i].User < out[j].User // stable tiebreak
	})

	if len(out) <= capacityMaxUsers {
		return out
	}
	// Fold everyone past the cap into a single "others" row (User == "").
	others := topology.CapacityUser{}
	for _, u := range out[capacityMaxUsers:] {
		others.MemBytes += u.MemBytes
		others.CPUMillis += u.CPUMillis
		others.Sessions += u.Sessions
	}
	return append(out[:capacityMaxUsers:capacityMaxUsers], others)
}

func shortOwnerID(id string) string {
	if len(id) > 6 {
		return id[:6]
	}
	return id
}

func notImplemented(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}
