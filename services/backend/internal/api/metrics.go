package api

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"backend/internal/labs"
)

// metricsHandler publishes the Prometheus text exposition format by hand.
//
// No client library on purpose. The backend carries no metrics dependency, the
// format is stable and trivial (HELP, TYPE, then name{labels} value), and these
// are a handful of gauges and counters - not worth a dependency tree on a
// platform whose whole subject is supply chain and isolation.
//
// What this deliberately does NOT try to be is node-level monitoring. It reports
// what the backend itself knows: sessions it is tracking, memory it has promised,
// and why it has turned people away. Actual node memory has to come from
// node_exporter reading /proc, because the kubelet's stats API returns 500 for
// the whole node whenever any Kata pod is running (cadvisor cannot stat a
// devmapper container), which is precisely when a lab platform needs it.
func metricsHandler(mgr *labs.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats := mgr.Stats()
		rejected, ended := labs.Counters()

		var b strings.Builder
		gauge := func(name, help string, value any) {
			fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s gauge\n%s %v\n", name, help, name, name, value)
		}

		fmt.Fprintf(&b, "# HELP k8labs_sessions_active Live lab sessions, by module.\n# TYPE k8labs_sessions_active gauge\n")
		// Sorted so the output is stable between scrapes - unstable ordering makes
		// diffing two scrapes by hand needlessly painful during an incident.
		for _, slug := range sortedKeys(stats.SessionsByModule) {
			fmt.Fprintf(&b, "k8labs_sessions_active{module=%q} %d\n", slug, stats.SessionsByModule[slug])
		}

		gauge("k8labs_capacity_used_bytes",
			"Memory reserved by live sessions (sum of SessionMemory).", stats.UsedBytes)
		gauge("k8labs_capacity_budget_bytes",
			"Admission ceiling; 0 means the capacity gate is disabled.", stats.BudgetBytes)
		gauge("k8labs_session_oldest_age_seconds",
			"Age of the longest-running session. Approaching the configured MaxAge means sessions are being held to their cap rather than finished.",
			int64(stats.OldestAge.Seconds()))

		fmt.Fprintf(&b, "# HELP k8labs_admission_rejected_total Session starts refused, by reason.\n# TYPE k8labs_admission_rejected_total counter\n")
		// Every reason is emitted even at zero. A counter that only appears once it
		// first fires cannot be alerted on with a rate() rule until the very event
		// you wanted the alert for has already happened.
		for _, reason := range []string{labs.ReasonUserSessionLimit, labs.ReasonDailyQuota, labs.ReasonNodeCapacity} {
			fmt.Fprintf(&b, "k8labs_admission_rejected_total{reason=%q} %d\n", reason, rejected[reason])
		}

		fmt.Fprintf(&b, "# HELP k8labs_sessions_ended_total Sessions torn down, by reason.\n# TYPE k8labs_sessions_ended_total counter\n")
		for _, reason := range []string{labs.EndReasonStopped, labs.EndReasonIdle, labs.EndReasonExpired} {
			fmt.Fprintf(&b, "k8labs_sessions_ended_total{reason=%q} %d\n", reason, ended[reason])
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		w.Write([]byte(b.String()))
	}
}

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
