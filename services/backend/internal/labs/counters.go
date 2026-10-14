package labs

import "sync"

// Process-lifetime counters for /metrics. Deliberately plain atomics behind a
// mutex-guarded map rather than a Prometheus client: the backend has no metrics
// dependency today, the exposition format is a handful of lines to write (see
// internal/api/metrics.go), and for a security-training platform not adding a
// dependency tree to publish six numbers is worth the twenty lines.
//
// Counters only ever increase and reset on restart, which is exactly what a
// Prometheus counter is - a scraper handles the reset correctly on its own.
var counters = struct {
	mu       sync.Mutex
	rejected map[string]uint64 // by admission reason
	ended    map[string]uint64 // by end reason
}{
	rejected: map[string]uint64{},
	ended:    map[string]uint64{},
}

func countRejected(reason string) {
	counters.mu.Lock()
	counters.rejected[reason]++
	counters.mu.Unlock()
}

func countEnded(reason string) {
	counters.mu.Lock()
	counters.ended[reason]++
	counters.mu.Unlock()
}

// Counters returns copies of the admission-rejection and session-end tallies,
// keyed by reason.
//
// These two are the ones worth alerting on. Sustained rejections mean the
// platform is turning students away - whether that is capacity, one user's cap,
// or quotas is exactly what the reason label answers. And the ratio of ended
// reasons is the feedback loop on the TTLs: mostly "idle" means the idle timeout
// is carrying the load, mostly "expired" means labs are shorter than the work.
func Counters() (rejected, ended map[string]uint64) {
	counters.mu.Lock()
	defer counters.mu.Unlock()
	rejected = make(map[string]uint64, len(counters.rejected))
	for k, v := range counters.rejected {
		rejected[k] = v
	}
	ended = make(map[string]uint64, len(counters.ended))
	for k, v := range counters.ended {
		ended[k] = v
	}
	return rejected, ended
}
