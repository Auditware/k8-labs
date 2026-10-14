package labs

import (
	"testing"
	"time"
)

// The resolution chain is admin override -> Module.Timing -> DefaultTiming.
// These cover the two lower rungs; the override rung lives in internal/accounts.
func TestModuleTimingUsesTheModulesOwnDefault(t *testing.T) {
	got := ModuleTiming("ns-escape")
	if got.MaxAge != 30*time.Minute {
		t.Fatalf("ns-escape MaxAge = %s, want 30m", got.MaxAge)
	}
}

func TestModuleTimingFallsBackForUnknownSlug(t *testing.T) {
	got := ModuleTiming("no-such-lab")
	if got != DefaultTiming {
		t.Fatalf("unknown slug resolved to %+v, want DefaultTiming %+v", got, DefaultTiming)
	}
}

// A module states only what differs. Declaring MaxAge must not silently zero the
// idle timeout - that would disable idle reaping for that lab and leak the
// capacity this whole change was meant to reclaim.
func TestModuleTimingInheritsIdleWhenOnlyMaxAgeIsSet(t *testing.T) {
	for _, slug := range []string{"ns-escape", "suid-privesc", "ebpf-detect"} {
		t.Run(slug, func(t *testing.T) {
			got := ModuleTiming(slug)
			if got.Idle != DefaultTiming.Idle {
				t.Fatalf("%s Idle = %s, want inherited %s", slug, got.Idle, DefaultTiming.Idle)
			}
			if got.Idle == 0 {
				t.Fatalf("%s has no idle timeout: an abandoned tab would hold its memory until MaxAge", slug)
			}
		})
	}
}

// An eBPF session reserves 2.5 GiB against a normal lab's 1.5 GiB, so its
// minutes are ~1.7x more expensive. It gets more time because the lab genuinely
// needs it, but the gap is a deliberate, bounded call rather than an accident.
func TestEBPFLabGetsLongerButBoundedDefault(t *testing.T) {
	ebpf := ModuleTiming("ebpf-detect").MaxAge
	normal := ModuleTiming("ns-escape").MaxAge
	if ebpf <= normal {
		t.Fatalf("ebpf MaxAge %s <= normal %s: the heavier lab should get the longer window", ebpf, normal)
	}
	if ebpf > 2*normal {
		t.Fatalf("ebpf MaxAge %s is more than twice normal %s: at 1.7x the memory cost that is a capacity problem", ebpf, normal)
	}
}

// Idle is the number that actually reclaims capacity, since a session holds its
// full reservation whether or not anyone is typing. Guard it from drifting back
// up without a deliberate decision.
func TestDefaultIdleStaysTight(t *testing.T) {
	if DefaultTiming.Idle > 15*time.Minute {
		t.Fatalf("DefaultTiming.Idle = %s: abandoned sessions hold 1.5-2.5 GiB for that long", DefaultTiming.Idle)
	}
	if DefaultTiming.Idle < MinDuration {
		t.Fatalf("DefaultTiming.Idle = %s is below MinDuration %s", DefaultTiming.Idle, MinDuration)
	}
}

// Every catalog module must resolve to something the reaper will honour, and
// idle must never exceed the hard cap or it can never fire.
func TestCatalogTimingsAreCoherent(t *testing.T) {
	for _, m := range ListModules() {
		t.Run(m.Slug, func(t *testing.T) {
			got := ModuleTiming(m.Slug)
			if got.MaxAge < MinDuration || got.MaxAge > MaxDuration {
				t.Fatalf("MaxAge %s outside [%s, %s]", got.MaxAge, MinDuration, MaxDuration)
			}
			if got.Idle > got.MaxAge {
				t.Fatalf("Idle %s > MaxAge %s: the idle timeout can never fire", got.Idle, got.MaxAge)
			}
		})
	}
}
