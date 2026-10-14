package labs

import (
	"encoding/json"
	"sort"
	"testing"
	"time"
)

// OwnerID gates get/delete/terminal access (see internal/api/routes.go) but
// must never reach a client - knowing a session exists shouldn't also reveal
// which user it belongs to.
func TestSessionOwnerIDNotSerialized(t *testing.T) {
	sess := Session{ID: "abc123", ModuleSlug: "ns-escape", Status: "running", OwnerID: "user-1"}

	body, err := json.Marshal(sess)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if _, present := decoded["OwnerID"]; present {
		t.Errorf("OwnerID leaked into JSON output: %s", body)
	}
	if _, present := decoded["ownerID"]; present {
		t.Errorf("OwnerID leaked into JSON output: %s", body)
	}
}

// The provisioning UI anchors its progress bar to the phase this exposes, so it
// has to reach the client under a stable key while starting - and stay absent
// before the first gate clears rather than serializing as an empty string.
func TestSessionPhaseSerialization(t *testing.T) {
	withPhase := Session{ID: "abc123", ModuleSlug: "ns-escape", Status: "starting", Phase: "foothold-ready"}
	body, err := json.Marshal(withPhase)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
	if decoded["phase"] != "foothold-ready" {
		t.Errorf("phase not serialized under \"phase\": %s", body)
	}

	// omitempty: a just-created session that hasn't cleared a gate yet must not
	// emit phase at all, so the client's "no real progress yet" fallback engages.
	fresh, _ := json.Marshal(Session{ID: "abc123", ModuleSlug: "ns-escape", Status: "starting"})
	var d2 map[string]any
	_ = json.Unmarshal(fresh, &d2)
	if _, present := d2["phase"]; present {
		t.Errorf("empty phase should be omitted, got: %s", fresh)
	}
}

func TestManagerGetReturnsOwnerID(t *testing.T) {
	m := &Manager{sessions: map[string]*Session{
		"abc123": {ID: "abc123", ModuleSlug: "ns-escape", Status: "running", OwnerID: "user-1"},
	}}

	sess, ok := m.Get("abc123")
	if !ok {
		t.Fatal("expected session to be found")
	}
	if sess.OwnerID != "user-1" {
		t.Errorf("expected OwnerID=user-1, got %q", sess.OwnerID)
	}
}

// expiredSessions backs the auto-stop policy: a hard cap regardless of
// activity, or idle timeout since the last real terminal input - see
// Session.CreatedAt/LastActivity and StartReaper's own doc comment for why
// it's these two fields, not just "the tab is open".
func TestExpiredSessions(t *testing.T) {
	now := time.Now()
	// Each session carries its own limits now (resolved from the module's
	// lab_settings at Create), so the reaper takes no global durations.
	tm := Timing{MaxAge: 60 * time.Minute, Idle: 3 * time.Minute}

	sessions := map[string]*Session{
		"fresh": {
			Status: "running", Timing: tm, CreatedAt: now.Add(-1 * time.Minute), LastActivity: now.Add(-30 * time.Second),
		},
		"idle-past-timeout": {
			// Created recently, but no input in over 3 minutes - active
			// typing earlier doesn't matter, what counts is how long it's
			// been since the last one.
			Status: "running", Timing: tm, CreatedAt: now.Add(-10 * time.Minute), LastActivity: now.Add(-4 * time.Minute),
		},
		"old-but-active": {
			// Still being typed in right now, but past the 60-minute cap -
			// the hard cap wins regardless of activity, by design.
			Status: "running", Timing: tm, CreatedAt: now.Add(-61 * time.Minute), LastActivity: now,
		},
		"starting-idle": {
			// Would qualify on idle time alone, but hasn't reached
			// "running" yet (still provisioning) - not reaped mid-start.
			Status: "starting", Timing: tm, CreatedAt: now.Add(-10 * time.Minute), LastActivity: now.Add(-10 * time.Minute),
		},
		"errored-idle": {
			// Same reasoning as starting-idle: a failed session, its own
			// Create already best-effort cleaned it up (see cleanup).
			Status: "error", Timing: tm, CreatedAt: now.Add(-10 * time.Minute), LastActivity: now.Add(-10 * time.Minute),
		},
		"short-lab-past-its-own-cap": {
			// A 10-minute lab, 12 minutes old. Under the old single global
			// cap (60m) this survived; per-session limits are the whole point
			// of this change, so it must now be reaped while the 60-minute
			// sessions above it are not.
			Status: "running", Timing: Timing{MaxAge: 10 * time.Minute, Idle: 3 * time.Minute},
			CreatedAt: now.Add(-12 * time.Minute), LastActivity: now,
		},
	}

	got := expiredSessions(sessions, now)
	ids := make([]string, 0, len(got))
	for _, e := range got {
		ids = append(ids, e.ID)
	}
	sort.Strings(ids)

	want := []string{"idle-past-timeout", "old-but-active", "short-lab-past-its-own-cap"}
	if len(ids) != len(want) {
		t.Fatalf("expiredSessions() = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Errorf("expiredSessions() = %v, want %v", ids, want)
			break
		}
	}
}

// The reason is what makes the usage row diagnostic rather than just an amount:
// a population reaped mostly on idle means something very different for TTL
// tuning than one reaped mostly on the hard cap.
func TestExpiredSessionsReportWhichClockFired(t *testing.T) {
	now := time.Now()
	sessions := map[string]*Session{
		"hit-hard-cap": {
			ID: "hit-hard-cap", Status: "running", Timing: Timing{MaxAge: 30 * time.Minute, Idle: 10 * time.Minute},
			CreatedAt: now.Add(-31 * time.Minute), LastActivity: now,
		},
		"went-idle": {
			ID: "went-idle", Status: "running", Timing: Timing{MaxAge: 30 * time.Minute, Idle: 10 * time.Minute},
			CreatedAt: now.Add(-15 * time.Minute), LastActivity: now.Add(-11 * time.Minute),
		},
	}

	byID := map[string]string{}
	for _, e := range expiredSessions(sessions, now) {
		byID[e.ID] = e.Reason
	}
	if byID["hit-hard-cap"] != EndReasonExpired {
		t.Errorf("hard-cap session reason = %q, want %q", byID["hit-hard-cap"], EndReasonExpired)
	}
	if byID["went-idle"] != EndReasonIdle {
		t.Errorf("idle session reason = %q, want %q", byID["went-idle"], EndReasonIdle)
	}
}

// A session past BOTH deadlines reports the hard cap: that is the promise the
// student was shown counting down, and it is the more informative of the two.
func TestExpiredSessionPrefersHardCapWhenBothFired(t *testing.T) {
	now := time.Now()
	sessions := map[string]*Session{
		"both": {
			ID: "both", Status: "running", Timing: Timing{MaxAge: 30 * time.Minute, Idle: 10 * time.Minute},
			CreatedAt: now.Add(-40 * time.Minute), LastActivity: now.Add(-20 * time.Minute),
		},
	}
	got := expiredSessions(sessions, now)
	if len(got) != 1 {
		t.Fatalf("expected exactly one expired session, got %d", len(got))
	}
	if got[0].Reason != EndReasonExpired {
		t.Fatalf("reason = %q, want %q", got[0].Reason, EndReasonExpired)
	}
}
