package labs

import (
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

// newAdmissionManager builds a Manager with no cluster behind it. admitLocked
// only reads m.sessions and m.cfg, so the nil clientset never comes into play.
func newAdmissionManager(t *testing.T, cfg Config) *Manager {
	t.Helper()
	return NewManager(nil, nil, cfg)
}

func (m *Manager) addTestSession(id, ownerID, slug string, timing Timing) {
	now := time.Now()
	m.sessions[id] = &Session{
		ID: id, OwnerID: ownerID, ModuleSlug: slug, Status: "running",
		CreatedAt: now, LastActivity: now, Timing: timing,
	}
}

var normalModule = Module{Slug: "ns-escape"}
var ebpfModule = Module{Slug: "ebpf-detect", EBPF: true}

func TestAdmissionRejectsSecondConcurrentSessionForSameUser(t *testing.T) {
	m := newAdmissionManager(t, Config{MaxSessionsPerUser: 1})
	m.addTestSession("s1", "alice", "ns-escape", DefaultTiming)

	err := m.admitLocked(normalModule, "alice")
	ae, ok := IsAdmission(err)
	if !ok {
		t.Fatalf("second session admitted, want refusal (got %v)", err)
	}
	if ae.Reason != ReasonUserSessionLimit {
		t.Fatalf("reason = %q, want %q", ae.Reason, ReasonUserSessionLimit)
	}
}

// The refusal must name one of the caller's existing sessions, so the UI can
// offer to stop that lab and start this one instead of dead-ending.
func TestAdmissionNamesTheConflictingSession(t *testing.T) {
	m := newAdmissionManager(t, Config{MaxSessionsPerUser: 1})
	m.addTestSession("s1", "alice", "ns-escape", DefaultTiming)

	err := m.admitLocked(ebpfModule, "alice") // starting a *different* lab
	ae, ok := IsAdmission(err)
	if !ok {
		t.Fatalf("want admission refusal, got %v", err)
	}
	if ae.ConflictSessionID != "s1" {
		t.Fatalf("ConflictSessionID = %q, want %q", ae.ConflictSessionID, "s1")
	}
	if ae.ConflictModule != "ns-escape" {
		t.Fatalf("ConflictModule = %q, want %q", ae.ConflictModule, "ns-escape")
	}
}

// The cap is per user, not global. One student at their limit must not block
// everybody else - that would turn a fairness rule into an outage.
func TestAdmissionIsolatesUsersFromEachOther(t *testing.T) {
	m := newAdmissionManager(t, Config{MaxSessionsPerUser: 1})
	m.addTestSession("s1", "alice", "ns-escape", DefaultTiming)

	if err := m.admitLocked(normalModule, "bob"); err != nil {
		t.Fatalf("bob refused because alice has a session: %v", err)
	}
}

func TestAdmissionAllowsConfiguredConcurrency(t *testing.T) {
	m := newAdmissionManager(t, Config{MaxSessionsPerUser: 3})
	m.addTestSession("s1", "alice", "ns-escape", DefaultTiming)
	m.addTestSession("s2", "alice", "ns-escape", DefaultTiming)

	if err := m.admitLocked(normalModule, "alice"); err != nil {
		t.Fatalf("third of three refused: %v", err)
	}
	m.addTestSession("s3", "alice", "ns-escape", DefaultTiming)
	if err := m.admitLocked(normalModule, "alice"); err == nil {
		t.Fatal("fourth session admitted with a limit of 3")
	}
}

func TestAdmissionDisabledWhenLimitIsZero(t *testing.T) {
	m := newAdmissionManager(t, Config{MaxSessionsPerUser: 0})
	for i, id := range []string{"s1", "s2", "s3"} {
		m.addTestSession(id, "alice", "ns-escape", DefaultTiming)
		if err := m.admitLocked(normalModule, "alice"); err != nil {
			t.Fatalf("session %d refused with the limit disabled: %v", i, err)
		}
	}
}

// The budget must refuse the session that would cross the line, not the one
// after it - admitting first and discovering the overrun later is the OOM.
func TestAdmissionRefusesSessionThatWouldExceedBudget(t *testing.T) {
	// Room for exactly two normal sessions (1.5 GiB each).
	m := newAdmissionManager(t, Config{MemoryBudget: resource.MustParse("3Gi")})
	m.addTestSession("s1", "alice", "ns-escape", DefaultTiming)
	m.addTestSession("s2", "bob", "ns-escape", DefaultTiming)

	err := m.admitLocked(normalModule, "carol")
	ae, ok := IsAdmission(err)
	if !ok {
		t.Fatalf("third session admitted into a two-session budget (got %v)", err)
	}
	if ae.Reason != ReasonNodeCapacity {
		t.Fatalf("reason = %q, want %q", ae.Reason, ReasonNodeCapacity)
	}
}

// An eBPF session reserves 2.5 GiB against a normal lab's 1.5 GiB, so it must
// consume the budget faster. Pricing them alike is how a node ends up
// oversubscribed by exactly the labs that can least afford it.
func TestAdmissionChargesEBPFSessionsMore(t *testing.T) {
	m := newAdmissionManager(t, Config{MemoryBudget: resource.MustParse("3Gi")})
	m.addTestSession("s1", "alice", "ebpf-detect", DefaultTiming)

	// 2.5 used of 3 GiB: another eBPF lab (2.5) cannot fit, and neither can a
	// normal one (1.5).
	if err := m.admitLocked(ebpfModule, "bob"); err == nil {
		t.Fatal("second eBPF session admitted: 5 GiB into a 3 GiB budget")
	}
	if err := m.admitLocked(normalModule, "bob"); err == nil {
		t.Fatal("normal session admitted: 4 GiB into a 3 GiB budget")
	}
}

// A pod adopted from an older catalog has a slug the registry no longer knows.
// Charging it nothing would let unknown sessions accumulate invisibly against a
// budget whose whole job is to be conservative.
func TestAdmissionChargesUnknownModulesTheHeavyRate(t *testing.T) {
	m := newAdmissionManager(t, Config{MemoryBudget: resource.MustParse("3Gi")})
	m.addTestSession("s1", "alice", "retired-lab-from-last-year", DefaultTiming)

	used := m.usedMemoryLocked()
	want := SessionMemory(Module{EBPF: true})
	if used.Cmp(want) != 0 {
		t.Fatalf("unknown module charged %s, want the heavy rate %s", used.String(), want.String())
	}
}

func TestAdmissionBudgetDisabledWhenZero(t *testing.T) {
	m := newAdmissionManager(t, Config{})
	for _, id := range []string{"s1", "s2", "s3", "s4"} {
		m.addTestSession(id, id, "ebpf-detect", DefaultTiming)
	}
	if err := m.admitLocked(ebpfModule, "eve"); err != nil {
		t.Fatalf("refused with no budget configured: %v", err)
	}
}

// Retry-After has to come from a real deadline. A capacity refusal that cannot
// say when to come back is only marginally better than a spinner.
func TestCapacityRefusalCarriesRetryAfter(t *testing.T) {
	m := newAdmissionManager(t, Config{MemoryBudget: resource.MustParse("1Gi")})
	m.addTestSession("s1", "alice", "ns-escape", Timing{MaxAge: 30 * time.Minute, Idle: 10 * time.Minute})

	err := m.admitLocked(normalModule, "bob")
	ae, ok := IsAdmission(err)
	if !ok {
		t.Fatalf("expected a capacity refusal, got %v", err)
	}
	if ae.RetryAfter <= 0 {
		t.Fatal("capacity refusal carries no RetryAfter")
	}
	// Bounded by the idle clock, which is the sooner of the two here.
	if ae.RetryAfter > 10*time.Minute {
		t.Fatalf("RetryAfter %s exceeds the soonest deadline (10m idle)", ae.RetryAfter)
	}
}

// The per-user cap is checked before the budget, so a student at their own
// limit is told that - not that the platform is full, which would be both
// confusing and untrue.
func TestUserLimitReportedBeforeCapacity(t *testing.T) {
	m := newAdmissionManager(t, Config{
		MaxSessionsPerUser: 1,
		MemoryBudget:       resource.MustParse("1Gi"), // already over
	})
	m.addTestSession("s1", "alice", "ns-escape", DefaultTiming)

	ae, ok := IsAdmission(m.admitLocked(normalModule, "alice"))
	if !ok {
		t.Fatal("expected a refusal")
	}
	if ae.Reason != ReasonUserSessionLimit {
		t.Fatalf("reason = %q, want the user's own limit to be reported first", ae.Reason)
	}
}

// stubQuota reports a fixed spend and an optional per-user override, so quota
// behaviour is testable without a database.
type stubQuota struct {
	used     float64
	override *float64
}

func (s stubQuota) UsedSince(string, time.Duration) float64 { return s.used }

func (s stubQuota) QuotaOverride(string) (float64, bool) {
	if s.override == nil {
		return 0, false
	}
	return *s.override, true
}

func floatPtr(v float64) *float64 { return &v }

func TestQuotaRefusesUserOverAllowance(t *testing.T) {
	m := newAdmissionManager(t, Config{DailyQuotaGiBMinutes: 100})
	m.quota = stubQuota{used: 120}

	ae, ok := IsAdmission(m.admitLocked(normalModule, "alice"))
	if !ok {
		t.Fatal("user over quota was admitted")
	}
	if ae.Reason != ReasonDailyQuota {
		t.Fatalf("reason = %q, want %q", ae.Reason, ReasonDailyQuota)
	}
}

func TestQuotaAdmitsUserUnderAllowance(t *testing.T) {
	m := newAdmissionManager(t, Config{DailyQuotaGiBMinutes: 100})
	m.quota = stubQuota{used: 40}

	if err := m.admitLocked(normalModule, "alice"); err != nil {
		t.Fatalf("user at 40 of 100 refused: %v", err)
	}
}

// Unset quota must not accidentally mean "zero allowance" - that would lock
// every student out the moment the feature shipped.
func TestQuotaDisabledWhenUnset(t *testing.T) {
	m := newAdmissionManager(t, Config{})
	m.quota = stubQuota{used: 99999}

	if err := m.admitLocked(normalModule, "alice"); err != nil {
		t.Fatalf("refused with no quota configured: %v", err)
	}
}

// A per-user override tightens the cap below the global default: a user given a
// smaller allowance is refused where the global would have admitted them.
func TestQuotaPerUserOverrideTightens(t *testing.T) {
	m := newAdmissionManager(t, Config{DailyQuotaGiBMinutes: 100})
	m.quota = stubQuota{used: 60, override: floatPtr(50)}

	ae, ok := IsAdmission(m.admitLocked(normalModule, "alice"))
	if !ok {
		t.Fatal("user over their 50 override was admitted")
	}
	if ae.Reason != ReasonDailyQuota {
		t.Fatalf("reason = %q, want %q", ae.Reason, ReasonDailyQuota)
	}
}

// A per-user override raises the cap above the global default: a heavy user
// given more room is admitted where the global would have refused them.
func TestQuotaPerUserOverrideRaises(t *testing.T) {
	m := newAdmissionManager(t, Config{DailyQuotaGiBMinutes: 100})
	m.quota = stubQuota{used: 120, override: floatPtr(200)}

	if err := m.admitLocked(normalModule, "alice"); err != nil {
		t.Fatalf("user at 120 of a 200 override refused: %v", err)
	}
}

// An override of 0 exempts a user entirely, even when a global quota is set and
// they have consumed a great deal.
func TestQuotaPerUserOverrideZeroExempts(t *testing.T) {
	m := newAdmissionManager(t, Config{DailyQuotaGiBMinutes: 100})
	m.quota = stubQuota{used: 99999, override: floatPtr(0)}

	if err := m.admitLocked(normalModule, "alice"); err != nil {
		t.Fatalf("user with a 0 (unlimited) override refused: %v", err)
	}
}

// GiB-minutes is memory reserved x wall-clock life. An eBPF lab must draw the
// allowance down faster for the same minutes, because it really does hold more.
func TestUsagePricesEBPFHigherForEqualTime(t *testing.T) {
	start := time.Now().Add(-30 * time.Minute)
	end := start.Add(30 * time.Minute)

	normal := usageFor(&Session{ID: "a", OwnerID: "u", ModuleSlug: "ns-escape", CreatedAt: start}, EndReasonStopped, end)
	ebpf := usageFor(&Session{ID: "b", OwnerID: "u", ModuleSlug: "ebpf-detect", CreatedAt: start}, EndReasonStopped, end)

	if ebpf.GiBMinutes <= normal.GiBMinutes {
		t.Fatalf("ebpf %.1f <= normal %.1f GiB-minutes for the same 30 minutes", ebpf.GiBMinutes, normal.GiBMinutes)
	}
	// 2.5 GiB vs 1.5 GiB reserved, so ~1.67x.
	ratio := ebpf.GiBMinutes / normal.GiBMinutes
	if ratio < 1.6 || ratio > 1.75 {
		t.Fatalf("ratio %.2f, want ~1.67 (2.5 GiB vs 1.5 GiB)", ratio)
	}
	// 1.5 GiB for 30 minutes = 45 GiB-minutes.
	if normal.GiBMinutes < 44 || normal.GiBMinutes > 46 {
		t.Fatalf("normal 30-minute lab = %.1f GiB-minutes, want ~45", normal.GiBMinutes)
	}
}

// Clock skew or an adopted session with a future CreatedAt must not produce
// negative usage, which would credit the user for running a lab.
func TestUsageNeverNegative(t *testing.T) {
	now := time.Now()
	rec := usageFor(&Session{ID: "a", OwnerID: "u", ModuleSlug: "ns-escape", CreatedAt: now.Add(time.Hour)}, EndReasonStopped, now)
	if rec.GiBMinutes < 0 {
		t.Fatalf("GiBMinutes = %.2f, want >= 0", rec.GiBMinutes)
	}
}
