package labs

import (
	"errors"
	"testing"
	"time"
)

// researchSession builds a running research-lab session created `age` ago with a
// MaxAge of `maxAge`, registered in a fresh test Manager. Returns both.
func researchSession(t *testing.T, age, maxAge time.Duration) (*Manager, *Session) {
	t.Helper()
	m := newTestManager()
	now := time.Now()
	s := &Session{
		ID:           "s1",
		ModuleSlug:   "driver-research-dev",
		Status:       "running",
		OwnerID:      "user-1",
		CreatedAt:    now.Add(-age),
		LastActivity: now,
		Timing:       Timing{MaxAge: maxAge, Idle: 30 * time.Minute},
	}
	m.sessions[s.ID] = s
	return m, s
}

func TestExtendPushesHardCap(t *testing.T) {
	m, s := researchSession(t, 0, 2*time.Hour)
	before := s.ExpiresAt()

	got, err := m.Extend("s1")
	if err != nil {
		t.Fatalf("Extend: %v", err)
	}
	want := before.Add(ExtendStep)
	if !got.Equal(want) {
		t.Fatalf("new expiry = %v, want %v", got, want)
	}
	if s.Timing.MaxAge != 4*time.Hour {
		t.Fatalf("MaxAge = %v, want 4h", s.Timing.MaxAge)
	}
}

func TestExtendResetsIdleClock(t *testing.T) {
	// A session that has gone 25 min without input (idle deadline 5 min away).
	// Extending must slide the idle deadline out too, not just the hard cap -
	// otherwise the idle reaper beats the freshly-extended hard cap to the kill.
	m, s := researchSession(t, 0, 2*time.Hour)
	s.LastActivity = time.Now().Add(-25 * time.Minute)
	staleIdle := s.IdleDeadline()

	if _, err := m.Extend("s1"); err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if !s.IdleDeadline().After(staleIdle) {
		t.Fatalf("idle deadline = %v, expected it slid forward from %v", s.IdleDeadline(), staleIdle)
	}
	// Full idle window restored (LastActivity is now, so ~Idle away).
	if left := time.Until(s.IdleDeadline()); left < 29*time.Minute {
		t.Fatalf("idle window after extend = %v, want ~30m", left)
	}
}

func TestExtendClampsToHardCapAndThenRefuses(t *testing.T) {
	// One step below the 48h ceiling: the next extend must land exactly on the
	// cap, not ExtendStep past it.
	m, s := researchSession(t, 0, ExtendHardCap-time.Hour)
	ceiling := s.CreatedAt.Add(ExtendHardCap)

	got, err := m.Extend("s1")
	if err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if !got.Equal(ceiling) {
		t.Fatalf("clamped expiry = %v, want the 48h ceiling %v", got, ceiling)
	}

	// Now at the cap - a further extend is refused rather than a no-op success.
	if _, err := m.Extend("s1"); !errors.Is(err, ErrAtHardCap) {
		t.Fatalf("Extend at cap = %v, want ErrAtHardCap", err)
	}
}

func TestExtendSlidesAIGrant(t *testing.T) {
	m, s := researchSession(t, 0, 2*time.Hour)
	// Grant expiring before the ceiling, so the slide is visible and not clamped.
	m.GrantAI(s.ID, "jti-1", time.Hour)
	before, ok := m.GrantExpiry(s.ID)
	if !ok {
		t.Fatal("GrantExpiry: no grant after GrantAI")
	}

	if _, err := m.Extend("s1"); err != nil {
		t.Fatalf("Extend: %v", err)
	}
	after, ok := m.GrantExpiry(s.ID)
	if !ok {
		t.Fatal("GrantExpiry: grant vanished after Extend")
	}
	if got := after.Sub(before); got != ExtendStep {
		t.Fatalf("grant slid by %v, want %v", got, ExtendStep)
	}
	// The pasted token stays valid across the extend - the whole point of the
	// in-place slide (no re-mint).
	if !m.VerifyAIGrant(s.ID, "jti-1") {
		t.Fatal("grant jti must still verify after Extend")
	}
}

func TestExtendGrantClampsToCeiling(t *testing.T) {
	// Session near the cap; a grant that would otherwise slide past the ceiling
	// must be clamped to it, not run past the session's own hard cap.
	m, s := researchSession(t, 0, ExtendHardCap-time.Hour)
	ceiling := s.CreatedAt.Add(ExtendHardCap)
	m.GrantAI(s.ID, "jti-1", ExtendHardCap) // expires ~ceiling already

	if _, err := m.Extend("s1"); err != nil {
		t.Fatalf("Extend: %v", err)
	}
	after, _ := m.GrantExpiry(s.ID)
	if after.After(ceiling) {
		t.Fatalf("grant expiry %v ran past the 48h ceiling %v", after, ceiling)
	}
}

func TestExtendRejectsNonResearchLab(t *testing.T) {
	m, s := researchSession(t, 0, 2*time.Hour)
	s.ModuleSlug = "ns-escape" // a guided lab, not extendable

	if _, err := m.Extend("s1"); !errors.Is(err, ErrNotExtendable) {
		t.Fatalf("Extend non-research = %v, want ErrNotExtendable", err)
	}
}

func TestExtendRejectsNotRunning(t *testing.T) {
	m, s := researchSession(t, 0, 2*time.Hour)
	s.Status = "starting"

	if _, err := m.Extend("s1"); !errors.Is(err, ErrNotRunning) {
		t.Fatalf("Extend not-running = %v, want ErrNotRunning", err)
	}
}

func TestExtendRejectsMissingSession(t *testing.T) {
	m := newTestManager()
	if _, err := m.Extend("nope"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Extend missing = %v, want ErrSessionNotFound", err)
	}
}
