package labs

import "testing"

// The driver picker flags an incompatible .ko before a session starts by
// comparing the module's vermagic to the lab's ExpectedVermagic. That only
// works if every lab which can mount user drivers actually carries the field -
// an empty one silently disables the check and hands the researcher back the
// opaque mid-session insmod failure it exists to prevent.
//
// Exactly the research labs mount drivers, so this asserts the field tracks
// IsResearch in both directions: a new research lab that forgets it fails here,
// and so does a non-research lab that grows one by accident.
func TestExpectedVermagicIsSetForResearchLabsOnly(t *testing.T) {
	for slug, m := range modules {
		switch {
		case m.IsResearch() && m.ExpectedVermagic == "":
			t.Errorf("%s: research lab has no ExpectedVermagic - the picker cannot flag an unloadable .ko", slug)
		case !m.IsResearch() && m.ExpectedVermagic != "":
			t.Errorf("%s: non-research lab carries ExpectedVermagic %q, but it mounts no drivers", slug, m.ExpectedVermagic)
		}
	}
}

// vermagic is matched against what `modinfo -F vermagic` prints for a module
// built in-session, so the pinned constant has to be in that exact shape:
// "<version> SMP mod_unload". A value with stray padding would make every
// driver read as mismatched, since the comparison is on the trimmed strings.
func TestResearchVermagicIsCanonical(t *testing.T) {
	if researchVermagic == "" {
		t.Fatal("researchVermagic is empty")
	}
	if got := researchVermagic; got != "6.18.35 SMP mod_unload" {
		// Not a correctness failure on its own - the kernel does get bumped - but
		// it must move in lockstep with the research Dockerfile's KVER and
		// infra/cluster/kata-fc-research-node.sh, so make the change deliberate.
		t.Errorf("researchVermagic is %q; if the research kernel was bumped, update KVER in "+
			"services/session-runner/research/Dockerfile and kata-fc-research-node.sh too", got)
	}
}
