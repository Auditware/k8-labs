package labs

// progressionTiers is the solve-gated unlock ladder for a regular user's
// catalog. Tier 0 is entitled to every authenticated user (the free first
// mission); each later tier unlocks only once every lab in the previous tier has
// been solved (the captured flag). Any lab left off the ladder entirely is
// explicit-grant only - an admin grants those, e.g. after the Telegram request
// from the "Encrypted" more-labs entry. As it stands every lab is on the ladder,
// so the funnel is pure progression: sign-up hands you one lab, solving it
// reveals the next few, and clearing those opens the rest - engagement, not a
// grant, is what unlocks the catalog.
//
// Kept beside the module catalog so the curriculum and its gating live together.
// The entitlement logic in internal/accounts consumes ProgressionTiers().
// TestProgressionSlugsAreRealModules asserts every slug here is a known module,
// so a typo or a since-removed lab fails the build rather than dangling.
var progressionTiers = [][]string{
	{"ns-escape"}, // tier 0: open to every signed-in user
	{"suid-privesc", "ebpf-detect", "sa-impersonate"}, // tier 1: unlocked by solving tier 0
	// tier 2: the rest of the catalog, unlocked by solving all of tier 1. With
	// every lab now on the ladder, finishing the free set opens everything - no
	// lab is explicit-grant-only anymore, so the admin-grant funnel is a no-op
	// unless a future lab is deliberately left out of this list.
	{
		"create-pods",
		"ebpf-egress",
		"hostpath-escape",
		"imagepullsecret-theft",
		"malicious-webhook",
		"node-token-harvest",
		"pod-exec",
		"privileged-escape",
		"recon-first-contact",
		"recon-neighborhood",
		"recon-who-am-i",
		"rolebinding-escalate",
		"sidecar-inject",
		"static-pod-persist",
	},
}

// ProgressionTiers returns the solve-gated unlock ladder (see progressionTiers).
// The returned slices are the package's own; treat them as read-only.
func ProgressionTiers() [][]string { return progressionTiers }
