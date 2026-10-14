package labs

import (
	"encoding/json"
	"testing"
	"time"
)

// Clamp is the last line of defence between an admin's number and the reaper.
// The cases that matter are the ones where accepting the input verbatim would
// produce a setting that silently does nothing.
func TestTimingClamp(t *testing.T) {
	cases := []struct {
		name     string
		in       Timing
		wantMax  time.Duration
		wantIdle time.Duration
	}{
		{
			name: "zero falls back to defaults, not to the minimum",
			// "unset" and "set to something tiny" are different intents - a
			// zero here means nobody configured it, so it must land on the
			// advertised 60m, not on MinDuration.
			in:      Timing{},
			wantMax: DefaultTiming.MaxAge, wantIdle: DefaultTiming.Idle,
		},
		{
			name:    "below the floor clamps up",
			in:      Timing{MaxAge: time.Minute, Idle: time.Minute},
			wantMax: MinDuration, wantIdle: MinDuration,
		},
		{
			name:    "above the ceiling clamps down",
			in:      Timing{MaxAge: 30 * 24 * time.Hour, Idle: 30 * 24 * time.Hour},
			wantMax: MaxDuration, wantIdle: MaxDuration,
		},
		{
			name: "idle longer than the cap is capped to it",
			// Otherwise the idle timeout can never fire and the admin is
			// looking at a setting that does nothing.
			in:      Timing{MaxAge: 30 * time.Minute, Idle: 45 * time.Minute},
			wantMax: 30 * time.Minute, wantIdle: 30 * time.Minute,
		},
		{
			name:    "a valid pair is left alone",
			in:      Timing{MaxAge: 90 * time.Minute, Idle: 15 * time.Minute},
			wantMax: 90 * time.Minute, wantIdle: 15 * time.Minute,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.Clamp()
			if got.MaxAge != tc.wantMax {
				t.Errorf("MaxAge = %v, want %v", got.MaxAge, tc.wantMax)
			}
			if got.Idle != tc.wantIdle {
				t.Errorf("Idle = %v, want %v", got.Idle, tc.wantIdle)
			}
		})
	}
}

// The two deadlines answer different questions and must not be conflated: the
// hard cap is fixed at create time, the idle deadline slides with every
// keystroke.
func TestSessionDeadlines(t *testing.T) {
	created := time.Now().Add(-10 * time.Minute)
	typed := time.Now().Add(-2 * time.Minute)
	s := Session{
		CreatedAt:    created,
		LastActivity: typed,
		Timing:       Timing{MaxAge: 60 * time.Minute, Idle: 20 * time.Minute},
	}

	if want := created.Add(60 * time.Minute); !s.ExpiresAt().Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v (must anchor on CreatedAt)", s.ExpiresAt(), want)
	}
	if want := typed.Add(20 * time.Minute); !s.IdleDeadline().Equal(want) {
		t.Errorf("IdleDeadline = %v, want %v (must anchor on LastActivity)", s.IdleDeadline(), want)
	}

	// Typing must push the idle deadline out without touching the hard cap -
	// the promise the student was shown can't move.
	hardCap := s.ExpiresAt()
	s.LastActivity = time.Now()
	if !s.ExpiresAt().Equal(hardCap) {
		t.Error("activity moved ExpiresAt; the hard cap must be fixed at create time")
	}
}

// The UI counts down to these, so they have to be on the wire. A regression
// here is silent: the countdown would just render NaN or 00:00.
func TestSessionJSONCarriesBothDeadlines(t *testing.T) {
	s := Session{
		ID: "abc", ModuleSlug: "ns-escape", Status: "running", OwnerID: "u1",
		CreatedAt: time.Now(), LastActivity: time.Now(), Timing: DefaultTiming,
	}
	blob, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(blob, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"expiresAt", "idleExpiresAt"} {
		if _, ok := got[key]; !ok {
			t.Errorf("session JSON missing %q: %s", key, blob)
		}
	}
	// The owner must still never be serialized - adding MarshalJSON is exactly
	// the kind of change that can quietly undo a `json:"-"` tag.
	if _, leaked := got["OwnerID"]; leaked {
		t.Error("MarshalJSON leaked OwnerID")
	}
	if _, leaked := got["ownerID"]; leaked {
		t.Error("MarshalJSON leaked ownerID")
	}
}
