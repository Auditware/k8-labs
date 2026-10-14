package labs

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Admission reasons. Stable machine keys: they are the metric label
// (admission_rejected_total{reason}) and the API's switch, so they must not be
// reworded casually even though the human message beside them can be.
const (
	ReasonUserSessionLimit = "user_session_limit"
	ReasonNodeCapacity     = "node_capacity"
	ReasonDailyQuota       = "daily_quota"
)

// AdmissionError is a refusal to start a session, carrying enough for the API
// to answer honestly: which rule said no, what to tell the student, and when it
// is worth trying again.
//
// The distinction matters to the person waiting. "You already have a lab open"
// is their own doing and instantly actionable; "the platform is full" is not
// their fault and only time fixes it. Collapsing both into one 429 would make
// the first look like a punishment and the second look like a bug.
type AdmissionError struct {
	Reason  string
	Message string
	// RetryAfter is when capacity is next expected, derived from the
	// soonest-expiring live session. Zero means unknown - the caller should omit
	// the header rather than invent a number.
	RetryAfter time.Duration
	// ConflictSessionID / ConflictModule identify one of the caller's existing
	// sessions when Reason is ReasonUserSessionLimit, so the UI can offer to stop
	// that lab and start this one in a single click instead of a dead-end error.
	// Empty for every other reason.
	ConflictSessionID string
	ConflictModule    string
}

func (e *AdmissionError) Error() string { return e.Message }

// IsAdmission reports whether err is a capacity/limit refusal rather than a
// provisioning failure, so handlers can map it to 4xx/503 instead of 500.
func IsAdmission(err error) (*AdmissionError, bool) {
	ae, ok := err.(*AdmissionError)
	return ae, ok
}

// admitLocked decides whether one more session may start. The caller MUST hold
// m.mu, and MUST insert the new session before releasing it.
//
// That is not incidental. Check and reserve have to be one atomic step or the
// limits leak under exactly the load they exist for: two requests arriving
// together would both read "0 sessions", both pass, and both start. Sessions are
// counted from m.sessions, which Reconcile rebuilds from live runner pods at
// startup, so the count survives a backend restart without a database table -
// the pods themselves are the source of truth.
func (m *Manager) admitLocked(module Module, ownerID string) error {
	if limit := m.cfg.MaxSessionsPerUser; limit > 0 {
		if n := m.userSessionsLocked(ownerID); n >= limit {
			countRejected(ReasonUserSessionLimit)
			ae := &AdmissionError{
				Reason: ReasonUserSessionLimit,
				Message: fmt.Sprintf(
					"You already have %s running. Stop it before starting another.",
					pluralSessions(n)),
			}
			// Name one existing session so the UI can offer to swap into this lab.
			if id, mod, ok := m.firstUserSessionLocked(ownerID); ok {
				ae.ConflictSessionID = id
				ae.ConflictModule = mod
			}
			return ae
		}
	}

	// Checked after the concurrency cap and before capacity, matching how
	// actionable each is to the person waiting: your own running lab first, then
	// your own allowance, then the platform being full.
	if used, over := m.quotaExceeded(ownerID); over {
		countRejected(ReasonDailyQuota)
		return &AdmissionError{
			Reason:  ReasonDailyQuota,
			Message: quotaMessage(used, m.cfg.DailyQuotaGiBMinutes),
		}
	}

	budget := m.cfg.MemoryBudget
	if budget.IsZero() {
		return nil
	}
	want := SessionMemory(module)
	used := m.usedMemoryLocked()
	after := used.DeepCopy()
	after.Add(want)
	if after.Cmp(budget) > 0 {
		countRejected(ReasonNodeCapacity)
		return &AdmissionError{
			Reason: ReasonNodeCapacity,
			Message: "Labs are at capacity right now. Your place is not lost - " +
				"try again shortly.",
			RetryAfter: m.nextExpiryLocked(),
		}
	}
	return nil
}

// userSessionsLocked counts one user's live sessions. Caller holds m.mu.
func (m *Manager) userSessionsLocked(ownerID string) int {
	n := 0
	for _, s := range m.sessions {
		if s.OwnerID == ownerID {
			n++
		}
	}
	return n
}

// firstUserSessionLocked returns one live session owned by ownerID - its id and
// module slug - for the "you already have X running" refusal to name and offer
// to stop. Map order is unspecified, so with a limit above 1 this is "some" of
// the caller's sessions, not a particular one; at the usual limit of 1 it is the
// only one. Caller holds m.mu.
func (m *Manager) firstUserSessionLocked(ownerID string) (id, module string, ok bool) {
	for _, s := range m.sessions {
		if s.OwnerID == ownerID {
			return s.ID, s.ModuleSlug, true
		}
	}
	return "", "", false
}

// usedMemoryLocked sums what every live session reserves. Caller holds m.mu.
//
// Sessions still "starting" count in full: their pod is already created (or
// about to be), so the memory is already committed as far as the scheduler is
// concerned. Counting only "running" would let a burst of simultaneous starts
// oversubscribe the node in the window before any of them finishes booting.
func (m *Manager) usedMemoryLocked() resource.Quantity {
	total := resource.Quantity{}
	for _, s := range m.sessions {
		// SessionMemoryBySlug charges an unknown slug (an adopted pod from an
		// older catalog) the heavier rate rather than nothing - undercounting is
		// what causes the OOM this whole path exists to prevent.
		total.Add(SessionMemoryBySlug(s.ModuleSlug))
	}
	return total
}

// nextExpiryLocked is how long until the soonest-expiring session frees its
// reservation, whichever of its two clocks fires first. Caller holds m.mu.
//
// It is an estimate on purpose: a student may stop a lab early (sooner) or keep
// typing and hold it to the hard cap (later, but bounded by MaxAge). Returning
// the earliest deadline gives a Retry-After that is honest about the best case
// without ever promising longer than the cap.
func (m *Manager) nextExpiryLocked() time.Duration {
	now := time.Now()
	var soonest time.Duration
	for _, s := range m.sessions {
		d := s.ExpiresAt().Sub(now)
		if idle := s.IdleDeadline().Sub(now); idle < d {
			d = idle
		}
		if d < 0 {
			// Already past its deadline and awaiting the next reaper sweep.
			d = 0
		}
		if soonest == 0 || d < soonest {
			soonest = d
		}
	}
	return soonest
}

func pluralSessions(n int) string {
	if n == 1 {
		return "a lab session"
	}
	return fmt.Sprintf("%d lab sessions", n)
}

// Stats is a point-in-time view of what the manager is holding, for /metrics
// and the admin dashboard. Values, not references, so callers cannot mutate
// live state.
type Stats struct {
	// SessionsByModule is live session count keyed by module slug.
	SessionsByModule map[string]int
	// UsedBytes is what those sessions reserve; BudgetBytes is the ceiling the
	// admission gate enforces (0 = disabled).
	UsedBytes   int64
	BudgetBytes int64
	// OldestAge is how long the longest-running session has been alive. A value
	// creeping toward the largest configured MaxAge is the signal that sessions
	// are being held to their cap rather than finished and released.
	OldestAge time.Duration
}

// Stats snapshots the manager. Cheap enough for a scrape interval: it walks the
// session map once under the lock and copies out primitives.
func (m *Manager) Stats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := Stats{SessionsByModule: make(map[string]int, len(m.sessions))}
	now := time.Now()
	for _, sess := range m.sessions {
		s.SessionsByModule[sess.ModuleSlug]++
		if age := now.Sub(sess.CreatedAt); age > s.OldestAge {
			s.OldestAge = age
		}
	}
	used := m.usedMemoryLocked()
	s.UsedBytes = used.Value()
	s.BudgetBytes = m.cfg.MemoryBudget.Value()
	return s
}
