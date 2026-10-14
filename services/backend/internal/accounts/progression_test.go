package accounts

import (
	"testing"

	"backend/internal/labs"
)

// progressionEntitled is the pure core of the solve-gated funnel. These cases
// pin the ladder against labs.ProgressionTiers so a change to the tiers that
// breaks the shape (tier 0 not open, a tier unlocking early) fails here.
func TestProgressionEntitled(t *testing.T) {
	tiers := labs.ProgressionTiers()
	if len(tiers) < 2 {
		t.Fatalf("test assumes at least two tiers, got %d", len(tiers))
	}
	tier0, tier1 := tiers[0], tiers[1]

	has := func(set map[string]bool, slugs []string) bool {
		for _, s := range slugs {
			if !set[s] {
				return false
			}
		}
		return true
	}
	hasAny := func(set map[string]bool, slugs []string) bool {
		for _, s := range slugs {
			if set[s] {
				return true
			}
		}
		return false
	}

	t.Run("no solves: only tier 0 is open", func(t *testing.T) {
		got := progressionEntitled(map[string]bool{})
		if !has(got, tier0) {
			t.Errorf("tier 0 must be open with no solves; got %v", got)
		}
		if hasAny(got, tier1) {
			t.Errorf("tier 1 must be locked until tier 0 is solved; got %v", got)
		}
	})

	t.Run("tier 0 solved: tier 1 unlocks", func(t *testing.T) {
		solved := map[string]bool{}
		for _, s := range tier0 {
			solved[s] = true
		}
		got := progressionEntitled(solved)
		if !has(got, tier0) || !has(got, tier1) {
			t.Errorf("solving tier 0 must open tiers 0 and 1; got %v", got)
		}
	})

	t.Run("solving every tier below the last opens the last tier", func(t *testing.T) {
		// Solve all tiers except the final one; the cascade must then open the
		// last. This is the generic form of "clearing tier N-1 unlocks tier N",
		// so it stays correct as tiers are added or removed.
		last := len(tiers) - 1
		solved := map[string]bool{}
		for i := 0; i < last; i++ {
			for _, s := range tiers[i] {
				solved[s] = true
			}
		}
		got := progressionEntitled(solved)
		if !has(got, tiers[last]) {
			t.Errorf("solving all tiers below the last must open the last tier %v; got %v", tiers[last], got)
		}
	})

	t.Run("solving the whole ladder entitles the whole ladder", func(t *testing.T) {
		solved := map[string]bool{}
		for _, tier := range tiers {
			for _, s := range tier {
				solved[s] = true
			}
		}
		got := progressionEntitled(solved)
		for i, tier := range tiers {
			if !has(got, tier) {
				t.Errorf("tier %d must be entitled once the ladder is solved; got %v", i, got)
			}
		}
	})

	t.Run("an unrelated solve does not unlock tier 1", func(t *testing.T) {
		got := progressionEntitled(map[string]bool{"some-other-lab": true})
		if hasAny(got, tier1) {
			t.Errorf("tier 1 must stay locked without all of tier 0 solved; got %v", got)
		}
	})
}

// entitledFrom is what the admin "lab access" column counts: explicit grants
// unioned with the free progression unlocks. These pin the property the admin
// page depends on - a user with no grant rows still shows the free first lab,
// not "No labs".
func TestEntitledFromFoldsFreeFirstLab(t *testing.T) {
	tier0 := labs.ProgressionTiers()[0]

	// No grants, nothing solved: still entitled to every tier-0 lab (the free
	// first mission), so the admin column never reads "No labs" for a real user.
	got := map[string]bool{}
	for _, s := range entitledFrom(nil, nil) {
		got[s] = true
	}
	for _, s := range tier0 {
		if !got[s] {
			t.Errorf("entitledFrom(nil,nil) missing free tier-0 lab %q; got %v", s, got)
		}
	}
}

func TestEntitledFromUnionsGrantsAndSolveUnlocks(t *testing.T) {
	tiers := labs.ProgressionTiers()
	if len(tiers) < 2 {
		t.Skip("needs at least two tiers")
	}
	// Solving all of tier 0 unlocks tier 1, and an explicit grant is folded in on
	// top - the exact union the admin access count must reflect.
	got := map[string]bool{}
	for _, s := range entitledFrom([]string{"hostpath-escape"}, tiers[0]) {
		got[s] = true
	}
	if !got["hostpath-escape"] {
		t.Errorf("explicit grant not reflected: %v", got)
	}
	for _, s := range tiers[1] {
		if !got[s] {
			t.Errorf("solving tier 0 should unlock tier-1 lab %q; got %v", s, got)
		}
	}
}
