package labs

import "testing"

// Every module must land in a known track, or the sidebar (which renders the
// catalog as track sections) would drop it into no group at all - it would
// silently vanish from the lab list. trackOf is the single source of that
// assignment, so this asserts it stays exhaustive as labs are added.
func TestEveryModuleHasAKnownTrack(t *testing.T) {
	known := map[string]bool{
		"first-flight": true,
		"recon":        true,
		"privesc":      true,
		"lateral":      true,
		"escape":       true,
		"persistence":  true,
		"research":     true,
	}
	for slug, m := range modules {
		if m.Track == "" {
			t.Errorf("%s: no Track assigned (add it to trackOf) - it would not appear under any sidebar group", slug)
			continue
		}
		if !known[m.Track] {
			t.Errorf("%s: Track %q is not one the sidebar renders", slug, m.Track)
		}
	}
}
