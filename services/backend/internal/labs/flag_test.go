package labs

import (
	"strings"
	"testing"
)

// Every module's Flag is the server-side source of truth the flag endpoint
// checks a submission against (see internal/api/progress.go). It has to be the
// exact value the module's manifest plants in the nested cluster - a k8labs{...}
// Secret, or a file dropped on the node - or the capstone would reject the flag
// a student legitimately captured. This asserts the two can never drift: the
// Flag must appear verbatim somewhere in the module's Manifests.
func TestModuleFlagMatchesManifest(t *testing.T) {
	for slug, m := range modules {
		if m.IsResearch() {
			continue // research labs are discovery sandboxes: no planted flag, no capstone
		}
		if m.Flag == "" {
			t.Errorf("%s: no Flag set - the flag capstone has nothing to check against", slug)
			continue
		}
		if !strings.HasPrefix(m.Flag, "k8labs{") || !strings.HasSuffix(m.Flag, "}") {
			t.Errorf("%s: Flag %q is not a k8labs{...} value", slug, m.Flag)
		}
		if m.Nested == nil {
			t.Errorf("%s: no Nested spec, so no manifest to plant the flag", slug)
			continue
		}
		if !strings.Contains(m.Nested.Manifests, m.Flag) {
			t.Errorf("%s: Flag %q does not appear in the module's manifests - "+
				"the value the student captures would not match what the endpoint checks", slug, m.Flag)
		}
	}
}

// Flags must be unique across modules: a submission is checked only against the
// module it was posted to, but a duplicated flag would mean two labs share a
// proof, and copying one lab's flag into another would silently "solve" it.
func TestModuleFlagsAreUnique(t *testing.T) {
	seen := map[string]string{}
	for slug, m := range modules {
		if m.IsResearch() {
			continue // no flag to collide
		}
		if other, dup := seen[m.Flag]; dup {
			t.Errorf("modules %s and %s share the flag %q", other, slug, m.Flag)
		}
		seen[m.Flag] = slug
	}
}
