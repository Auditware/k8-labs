// Command server is the backend's entrypoint: HTTP API for the module catalog,
// auth, and tenant session lifecycle. See docs/architecture/tech-stack.md.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"backend/internal/accounts"
	"backend/internal/api"
	"backend/internal/buildinfo"
	"backend/internal/config"
	"backend/internal/db"
	"backend/internal/health"
	"backend/internal/labs"
	"backend/internal/topology"

	"k8s.io/client-go/kubernetes"

	// Blank import - registers the generated spec (see Makefile's `swagger`
	// target) into swaggo's global registry so internal/api's swagger-ui
	// route (httpSwagger.Handler) has something to serve. Regenerate with
	// `make swagger` whenever a handler's annotations change - nothing
	// does this automatically, same "no real migration tool yet" caveat
	// as internal/db/schema.go.
	_ "backend/docs"
)

// @title K8 Labs API
// @description Backend API for the lab catalog, auth, and session lifecycle. See docs/architecture/tech-stack.md for why this is stdlib net/http, not a framework.
// @version 0.1.0
// @BasePath /api/v1
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer <token> (see internal/api/auth.go). The same JWT also works as an HttpOnly cookie for browser requests, but that can't be expressed as a header-based security scheme here.
func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)
	// First line every backend prints: which commit this binary is. Makes a
	// stale deploy (or a dead Tilt serving an old binary) visible in the logs,
	// and matches the sha /healthz reports. See internal/buildinfo.
	slog.Info("backend starting", "sha", buildinfo.SHA)

	// Load Vault-injected secrets first, if present - see internal/config
	// for why this is a manual parse rather than a shell `source` (image
	// is distroless, no shell). No-op locally/docker-compose.
	if err := config.LoadVaultSecrets(); err != nil {
		slog.Error("failed to load vault-injected secrets", "error", err)
		os.Exit(1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "7374"
	}

	pool, err := db.Connect()
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.EnsureSchema(context.Background(), pool); err != nil {
		slog.Error("schema bootstrap failed", "error", err)
		os.Exit(1)
	}

	// Showcase operators for the leaderboard/landing, planted only into a fresh
	// (empty) database - on an established one this is a no-op, so an admin who
	// deletes a seed account keeps it deleted (see db.SeedDefaultUsers). Non-fatal:
	// this is demo data, not a boot dependency - a seed hiccup should never keep
	// the backend from serving.
	if err := db.SeedDefaultUsers(context.Background(), pool); err != nil {
		slog.Warn("seeding default users failed", "error", err)
	}

	accountsCfg := accounts.ConfigFromEnv()

	// Session provisioning needs to run inside the cluster it manages - not
	// available in docker-compose/local dev, degrades to a clear per-request
	// error rather than failing the whole process (see internal/labs).
	// k8s stays a nil interface (not a typed-nil *Clientset) when there's no
	// in-cluster config, so the Manager's `clientset == nil` guards work.
	var k8s kubernetes.Interface
	clientset, restCfg, err := labs.NewClientset()
	if err != nil {
		slog.Warn("no in-cluster k8s config - session provisioning disabled", "error", err)
	} else {
		k8s = clientset
	}
	labsMgr := labs.NewManager(k8s, restCfg, labs.ConfigFromEnv())
	healthChecker := health.NewChecker(clientset)
	topoBuilder := topology.NewBuilder(clientset, restCfg)

	// Per-lab session limits (Admin -> Lab Management), resolved from
	// lab_settings at session-create time - see internal/labs/timing.go. Wired
	// BEFORE Reconcile on purpose: a session adopted without it would carry the
	// built-in default rather than its module's configured cap, so a
	// restart would silently change how long an in-flight session lives.
	timingStore := accounts.NewTimingStore(pool)
	labsMgr.SetTimingSource(timingStore)

	// Metering and the daily quota. Wired before Reconcile and StartReaper: the
	// reaper is what closes sessions, and a sweep that ran before the recorder
	// existed would tear sessions down without ever accounting for them.
	usageStore := accounts.NewUsageStore(pool)
	labsMgr.SetUsageRecorder(usageStore)
	labsMgr.SetQuotaSource(usageStore)

	// Adopts any runner pods already running from before this process started
	// (a dev-loop restart, a crash, a rollout) - without this, StartReaper
	// below can never see them, since it only looks at the in-memory
	// session map a restart just wiped. Must run before StartReaper.
	if err := labsMgr.Reconcile(context.Background()); err != nil {
		slog.Error("session reconcile failed", "error", err)
	}

	// Auto-stop policy for lab sessions. The limits are now PER LAB, resolved
	// at session-create time from lab_settings (Admin -> Lab Management) and
	// copied onto the session - see internal/labs/timing.go. The built-in
	// default is a hard 60-minute cap regardless of activity, or 20 minutes
	// with no real terminal input.
	//
	// Closing the browser tab is a faster, best-effort path on top of this (see
	// LabPage.tsx's pagehide handler), not what correctness here depends on.
	// context.Background(), not tied to shutdown - this exits with the process
	// same as everything else in-memory here.
	//
	// The idle window was 3 minutes while capacity was a black box; the admin
	// topology dashboard (internal/topology) now makes running sessions and
	// labs-node headroom visible, so it was relaxed to 20 - long enough that a
	// student reading the course or thinking mid-lab isn't reaped out from
	// under them, still short enough that a forgotten tab frees the microVM
	// well before the hard cap. (Verified live at a temporary 30s/10s setting:
	// a restart-orphaned session was both reconciled and correctly reaped end
	// to end.)
	// 10s sweep, not 30s: this only bounds how long an idle/expired session
	// lingers after its deadline passes - and that lag is visible, since the
	// admin topology map keeps showing the dead session until the sweep drops
	// it (an explicit Stop is immediate by comparison). Cheap to shorten: a
	// sweep is a mutex + a walk of the in-memory session map comparing
	// timestamps, and only touches Kubernetes when something has actually
	// expired, so tripling the rate costs nothing measurable.
	labsMgr.StartReaper(context.Background(), 10*time.Second)

	mux := api.NewRouter(pool, labsMgr, accountsCfg, healthChecker, topoBuilder, timingStore)

	srv := &http.Server{
		Addr:        ":" + port,
		Handler:     mux,
		ReadTimeout: 10 * time.Second,
		// Session creation waits synchronously for the pod to become ready
		// (up to 60s, see internal/labs.waitForReady) - needs real headroom
		// beyond a typical request, or the server cuts the response off
		// mid-provision and Traefik reports a 502 despite the pod actually
		// coming up fine.
		WriteTimeout: 90 * time.Second,
	}

	go func() {
		slog.Info("listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown on SIGTERM (how k8s asks a pod to stop) - matters here
	// specifically because in-flight session-provisioning requests shouldn't
	// be dropped mid-call during a rollout.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	<-stop

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}
