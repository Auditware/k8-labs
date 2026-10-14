package accounts

import (
	"sync"
	"time"

	"backend/internal/labs"
)

// timingCache is a tiny TTL cache in front of lab_settings. Bounded by the
// number of modules (single digits), so there is no eviction policy beyond the
// TTL - it cannot grow with traffic, only with the catalog.
//
// now is injectable so the TTL behaviour is testable without sleeping.
type timingCache struct {
	mu      sync.RWMutex
	entries map[string]timingEntry
	ttl     time.Duration
	now     func() time.Time
}

type timingEntry struct {
	timing labs.Timing
	expiry time.Time
}

func newTimingCache(ttl time.Duration) *timingCache {
	return &timingCache{entries: map[string]timingEntry{}, ttl: ttl, now: time.Now}
}

func (c *timingCache) get(slug string) (labs.Timing, bool) {
	c.mu.RLock()
	e, ok := c.entries[slug]
	c.mu.RUnlock()
	if !ok || c.now().After(e.expiry) {
		return labs.Timing{}, false
	}
	return e.timing, true
}

func (c *timingCache) put(slug string, t labs.Timing) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[slug] = timingEntry{timing: t, expiry: c.now().Add(c.ttl)}
}

func (c *timingCache) drop(slug string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, slug)
}
