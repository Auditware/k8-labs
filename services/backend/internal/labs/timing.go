package labs

import (
	"encoding/json"
	"time"
)

// Timing is how long one session may live. Two independent clocks, both
// enforced by the reaper (see StartReaper / expiredSessions):
//
//   - MaxAge: hard cap from CreatedAt, regardless of activity. This is the one
//     the lab UI counts down to, so it's a promise to the student.
//   - Idle: time since the last real terminal input. A student reading the
//     course for half an hour without typing is idle by this measure, which is
//     why the UI has to warn about it separately rather than showing only the
//     MaxAge countdown - otherwise a session can vanish with 38:00 still on the
//     clock.
type Timing struct {
	MaxAge time.Duration
	Idle   time.Duration
}

// DefaultTiming is the last-resort fallback: a module that declares no Timing
// of its own and has no admin override in lab_settings runs on this.
//
// Idle is 10 minutes rather than the 60 it once shared with MaxAge, and it is
// the more important of the two numbers. A session holds its full memory
// reservation - 1.5 GiB, or 2.5 GiB for an eBPF lab - whether or not anyone is
// typing into it, so an abandoned tab is indistinguishable from an active
// student in every way except cost. Idle time is where nearly all the waste
// was, and halving it roughly doubles effective capacity without shortening a
// single lab anybody is actually using.
//
// Prefer setting Module.Timing over changing this: lab length should follow how
// long a lab takes, and a global number cannot know that.
var DefaultTiming = Timing{MaxAge: 60 * time.Minute, Idle: 10 * time.Minute}

// ModuleTiming is a module's own declared default - the middle layer of the
// three-step resolution the reaper ultimately runs on:
//
//	admin override (lab_settings)  ->  Module.Timing  ->  DefaultTiming
//
// It exists so a lab's length can be a property of the lab, checked in with the
// lab, rather than a row somebody has to remember to create. An unknown slug or
// a module that declares nothing falls through to DefaultTiming.
func ModuleTiming(moduleSlug string) Timing {
	if m, ok := GetModule(moduleSlug); ok && m.Timing != nil {
		return m.Timing.Clamp()
	}
	return DefaultTiming
}

// Bounds on what an admin may set, enforced at the API layer as well as here.
// The floor keeps a typo from making every session unusable; the ceiling keeps
// one from pinning a runner (and its microVM) for a week.
const (
	MinDuration = 5 * time.Minute
	MaxDuration = 8 * time.Hour
)

// Clamp brings an override inside the supported range. A zero field falls back
// to the default rather than clamping up to MinDuration - "unset" and "set to
// something tiny" are different intents.
func (t Timing) Clamp() Timing {
	out := t
	if out.MaxAge == 0 {
		out.MaxAge = DefaultTiming.MaxAge
	}
	if out.Idle == 0 {
		out.Idle = DefaultTiming.Idle
	}
	out.MaxAge = clampDuration(out.MaxAge)
	out.Idle = clampDuration(out.Idle)
	// An idle timeout longer than the hard cap can never fire, which makes the
	// setting silently meaningless. Cap it so what an admin sees is what runs.
	if out.Idle > out.MaxAge {
		out.Idle = out.MaxAge
	}
	return out
}

func clampDuration(d time.Duration) time.Duration {
	if d < MinDuration {
		return MinDuration
	}
	if d > MaxDuration {
		return MaxDuration
	}
	return d
}

// TimingSource resolves a module's timing. Implemented by the Postgres-backed
// store (internal/accounts) and injected into the Manager, so this package
// keeps no database dependency. A nil source means "always defaults", which is
// what local dev without a database gets.
type TimingSource interface {
	TimingFor(moduleSlug string) Timing
}

// timingFor resolves a module's timing. With no source wired (local dev without
// a database) the module's own default stands; otherwise the source resolves
// the full override -> module -> global chain and this clamps the result.
func (m *Manager) timingFor(moduleSlug string) Timing {
	if m.timing == nil {
		return ModuleTiming(moduleSlug)
	}
	return m.timing.TimingFor(moduleSlug).Clamp()
}

// ExpiresAt is when the hard cap kills this session - the value the UI counts
// down to. Not the idle deadline, which moves every time the student types; see
// IdleDeadline.
func (s Session) ExpiresAt() time.Time {
	return s.CreatedAt.Add(s.Timing.MaxAge)
}

// IdleDeadline is when this session would be reaped if the student never typed
// again. Always <= ExpiresAt in practice (Timing.Clamp guarantees Idle <=
// MaxAge), and it slides forward on every keystroke.
func (s Session) IdleDeadline() time.Time {
	return s.LastActivity.Add(s.Timing.Idle)
}

// MarshalJSON adds the two deadlines the lab UI needs. They're derived, not
// stored, so they can't be plain struct tags - and they must come from here
// rather than being computed in the browser: a client-side countdown off a
// locally-remembered start time drifts, survives no device change, and is
// trivially editable, none of which is acceptable for a number that claims to
// say when the student's work disappears.
//
// Both are sent because they answer different questions. expiresAt is the
// promise ("this lab is 60 minutes"); idleExpiresAt moves forward on every
// keystroke and is what actually reaps a student who is reading rather than
// typing. A UI showing only the first would let a session vanish with half an
// hour apparently left.
func (s Session) MarshalJSON() ([]byte, error) {
	type sessionAlias Session // no methods, so no infinite recursion
	return json.Marshal(struct {
		sessionAlias
		ExpiresAt     time.Time `json:"expiresAt"`
		IdleExpiresAt time.Time `json:"idleExpiresAt"`
	}{
		sessionAlias:  sessionAlias(s),
		ExpiresAt:     s.ExpiresAt(),
		IdleExpiresAt: s.IdleDeadline(),
	})
}
