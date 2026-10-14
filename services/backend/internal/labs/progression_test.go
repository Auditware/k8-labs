package labs

import "testing"

// Every slug in the progression ladder must be a real module, and no slug may
// appear twice. A typo or a since-removed lab would otherwise silently create a
// tier that entitles nothing (or double-counts), quietly breaking the funnel.
func TestProgressionSlugsAreRealModules(t *testing.T) {
	seen := map[string]bool{}
	for i, tier := range ProgressionTiers() {
		for _, slug := range tier {
			if _, ok := GetModule(slug); !ok {
				t.Errorf("tier %d: %q is not a known module", i, slug)
			}
			if seen[slug] {
				t.Errorf("tier %d: %q appears in more than one tier", i, slug)
			}
			seen[slug] = true
		}
	}
}

// Tier 0 must be exactly one lab - the single free first mission the whole
// funnel is built around. If this ever needs to change, change it deliberately.
func TestProgressionTierZeroIsSingleLab(t *testing.T) {
	tiers := ProgressionTiers()
	if len(tiers) == 0 || len(tiers[0]) != 1 {
		t.Fatalf("tier 0 must be exactly one lab, got %v", tiers)
	}
}
