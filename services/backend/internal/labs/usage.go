package labs

import (
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

// End reasons, recorded with every session so the shape of normal use is
// visible. Mostly this answers "are the TTLs anywhere near right": lots of
// EndReasonExpired means labs are too short, lots of EndReasonIdle means the
// idle timeout is doing the real work, and lots of EndReasonStopped means
// students finish and leave cleanly.
const (
	EndReasonStopped = "stopped" // the student (or an admin) ended it
	EndReasonIdle    = "idle"    // reaped: no terminal input within Timing.Idle
	EndReasonExpired = "expired" // reaped: hit Timing.MaxAge
)

// UsageRecord is one finished session's cost, in raw components.
//
// GiBMinutes is the memory reservation multiplied by wall-clock life, and it is
// the honest unit because it measures what the session denied to everybody else
// - a reservation is held for the session's whole life whether or not anyone is
// typing into it. That makes an eBPF lab cost about 1.67x a normal one per
// minute, which is a measured ratio rather than a pricing decision.
//
// CPU is deliberately absent from the cost. It is time-shared and elastic:
// contention slows sessions briefly, it does not deny anyone a seat, and
// measured idle draw sits 4-8x BELOW the reservation. Charging for it would bill
// people for capacity nobody was refused. CPU belongs in abuse detection, where
// the control is the pod's CPU limit rather than the bill.
type UsageRecord struct {
	SessionID  string
	UserID     string
	ModuleSlug string
	StartedAt  time.Time
	EndedAt    time.Time
	GiBMinutes float64
	Reason     string
}

// UsageRecorder receives finished sessions. Implemented by the Postgres-backed
// store in internal/accounts, so this package keeps no database dependency -
// same seam as TimingSource. A nil recorder means "do not meter", which is what
// local dev without a database gets.
type UsageRecorder interface {
	RecordSession(rec UsageRecord)
}

// QuotaSource answers how much a user has consumed in the trailing window, in
// GiB-minutes, and whether that user has a per-user quota override. Nil means no
// quota, same as above.
type QuotaSource interface {
	UsedSince(userID string, window time.Duration) float64
	// QuotaOverride returns a per-user cap (GiB-minutes) and whether one is set.
	// false = no override, use the global default. A returned 0 is a real value
	// (unlimited for that user), distinct from the false case.
	QuotaOverride(userID string) (float64, bool)
}

// usageFor prices a session that has just ended.
//
// Wall-clock life, not "active" time. A session reserves its memory from the
// moment its pod is created until it is deleted; there is no partial
// reservation for a student who stepped away, so there is no honest way to
// discount those minutes. The idle reaper is what limits that exposure, and it
// does so by ending the session rather than by making it cheaper.
func usageFor(s *Session, reason string, now time.Time) UsageRecord {
	minutes := now.Sub(s.CreatedAt).Minutes()
	if minutes < 0 {
		minutes = 0
	}
	mem := SessionMemory(Module{EBPF: false})
	if module, ok := GetModule(s.ModuleSlug); ok {
		mem = SessionMemory(module)
	}
	gib := float64(mem.Value()) / (1 << 30)
	return UsageRecord{
		SessionID:  s.ID,
		UserID:     s.OwnerID,
		ModuleSlug: s.ModuleSlug,
		StartedAt:  s.CreatedAt,
		EndedAt:    now,
		GiBMinutes: gib * minutes,
		Reason:     reason,
	}
}

// SessionMemoryBySlug is SessionMemory resolved from a slug, for callers that
// hold only the slug (the topology builder, the live-usage rollup). An unknown
// slug is charged the heavy rate - the same conservative default the admission
// budget uses, so nothing is ever undercounted.
func SessionMemoryBySlug(slug string) resource.Quantity {
	if m, ok := GetModule(slug); ok {
		return SessionMemory(m)
	}
	return SessionMemory(Module{EBPF: true})
}

// SessionCPUMillis is the CPU request one session of this module carries, in
// millicores - the mirror of SessionMemory for the capacity summary. Kept in
// step with runnerResources: normal 250m, eBPF 500m.
func SessionCPUMillis(module Module) int64 {
	if module.usesEBPFKernel() {
		return 500
	}
	return 250
}

// SessionCPUMillisBySlug resolves SessionCPUMillis from a slug, charging an
// unknown slug the heavier rate - same conservative default as the memory side.
func SessionCPUMillisBySlug(slug string) int64 {
	if m, ok := GetModule(slug); ok {
		return SessionCPUMillis(m)
	}
	return SessionCPUMillis(Module{EBPF: true})
}

// LiveUsage is one user's IN-FLIGHT consumption: sessions still running, right
// now. It is the live complement to the historical lab_usage rollup, which only
// gains a row when a session ends.
//
// Both exist because they answer different questions and a student wants to see
// the first one while working - the historical view is "-" for the entire life
// of a session and then jumps, which reads like nothing is being counted.
// GiBMinutes here is accrued-so-far (reservation x minutes since start), the
// same unit the finished row will eventually record, so the number grows
// smoothly into its final value rather than appearing from nowhere.
type LiveUsage struct {
	Sessions      int     `json:"sessions"`
	ReservedBytes int64   `json:"reservedBytes"`
	GiBMinutes    float64 `json:"gibMinutes"`
}

// LiveUsageByUser rolls up every running session by owner. Cheap: one pass over
// the session map under the lock.
func (m *Manager) LiveUsageByUser() map[string]LiveUsage {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	out := map[string]LiveUsage{}
	for _, s := range m.sessions {
		if s.OwnerID == "" {
			continue
		}
		mem := SessionMemoryBySlug(s.ModuleSlug)
		bytes := mem.Value()
		minutes := now.Sub(s.CreatedAt).Minutes()
		if minutes < 0 {
			minutes = 0
		}
		lu := out[s.OwnerID]
		lu.Sessions++
		lu.ReservedBytes += bytes
		lu.GiBMinutes += float64(bytes) / (1 << 30) * minutes
		out[s.OwnerID] = lu
	}
	return out
}

// DailyQuotaGiBMinutes is the configured per-user daily allowance (0 = off), so
// the admin UI can render usage as a fraction of the cap. Read-only accessor;
// the value itself is set from config at startup.
func (m *Manager) DailyQuotaGiBMinutes() float64 {
	return m.cfg.DailyQuotaGiBMinutes
}

// SetUsageRecorder wires session metering. Call once at startup alongside
// SetTimingSource; leaving it unset disables metering entirely.
func (m *Manager) SetUsageRecorder(rec UsageRecorder) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.usage = rec
}

// SetQuotaSource wires the daily quota lookup. Leaving it unset disables the
// quota, which is what local dev without a database gets.
func (m *Manager) SetQuotaSource(q QuotaSource) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.quota = q
}

// recordUsage meters a finished session. Safe to call with no recorder wired.
// Runs synchronously: the caller has already removed the session, so there is
// no lock held and nothing to race with.
func (m *Manager) recordUsage(s *Session, reason string) {
	m.mu.Lock()
	rec := m.usage
	m.mu.Unlock()
	if rec == nil || s == nil || s.OwnerID == "" {
		return
	}
	rec.RecordSession(usageFor(s, reason, time.Now()))
}

// quotaExceeded reports whether this user is over their trailing-window
// allowance. Caller holds m.mu.
//
// Checked against usage ALREADY RECORDED, so a session in flight does not count
// until it ends. That is intentional and not a hole: concurrent use is bounded
// separately by MaxSessionsPerUser, and a single running session is bounded by
// its own MaxAge, so the in-flight exposure is one session's worth at most.
func (m *Manager) quotaExceeded(ownerID string) (float64, bool) {
	if m.quota == nil || ownerID == "" {
		return 0, false
	}
	// Effective cap: the user's own override if set, otherwise the global
	// default. A cap of 0 or less - whether the global is off or the override
	// exempts this user - means no quota, so nothing is refused.
	cap := m.cfg.DailyQuotaGiBMinutes
	if override, ok := m.quota.QuotaOverride(ownerID); ok {
		cap = override
	}
	if cap <= 0 {
		return 0, false
	}
	used := m.quota.UsedSince(ownerID, QuotaWindow)
	return used, used >= cap
}

// QuotaWindow is the trailing period the quota measures over. Rolling rather
// than calendar-day: a midnight reset is both gameable (burn it at 23:00, get a
// fresh allowance at 00:00) and unfair to whoever is not in the server's
// timezone.
const QuotaWindow = 24 * time.Hour

func quotaMessage(used, limit float64) string {
	return fmt.Sprintf(
		"You have used %.0f of your %.0f GiB-minutes for the day. "+
			"Your allowance frees up gradually as earlier sessions age out.",
		used, limit)
}
