package labs

import "testing"

// The catalog is stored in a map, and Go randomises map iteration on purpose.
// ListModules used to range that map straight into a slice, so it returned a
// different order on every call - and since the sidebar renders the array as-is,
// the lab list reshuffled whenever its query refetched (picking a lab did it).
//
// One call cannot catch that: any single ordering looks fine. These tests call
// it repeatedly and compare, which is what makes the randomisation observable.

func TestListModulesOrderIsStableAcrossCalls(t *testing.T) {
	first := ListModules()
	if len(first) < 2 {
		t.Fatalf("catalog has %d modules; need at least 2 for order to mean anything", len(first))
	}
	for i := 0; i < 50; i++ {
		got := ListModules()
		if len(got) != len(first) {
			t.Fatalf("call %d returned %d modules, want %d", i, len(got), len(first))
		}
		for j := range got {
			if got[j].Slug != first[j].Slug {
				t.Fatalf("call %d position %d = %q, want %q (catalog order is not stable)",
					i, j, got[j].Slug, first[j].Slug)
			}
		}
	}
}

func TestListModulesIsSortedByOrderThenSlug(t *testing.T) {
	got := ListModules()
	for i := 1; i < len(got); i++ {
		prev, cur := got[i-1], got[i]
		if prev.Order > cur.Order {
			t.Errorf("position %d (%s, order %d) sorts after %s (order %d)",
				i, cur.Slug, cur.Order, prev.Slug, prev.Order)
		}
		if prev.Order == cur.Order && prev.Slug > cur.Slug {
			t.Errorf("equal Order %d not tie-broken by slug: %q before %q", cur.Order, prev.Slug, cur.Slug)
		}
	}
}

// The sidebar order (Order 10..130) is a learning path, climbing in difficulty
// within each act and keeping dependent labs adjacent:
//   1. First Flight - a deliberate mixed sampler: the permission model, a local
//      privesc, detecting it, and an impersonation (ns-escape, suid-privesc,
//      ebpf-detect: ebpf reuses suid's exploit; sa-impersonate).
//   2. Recon (Order 41-43) - what an attacker does the moment they land on a
//      pod, before any escalation: read the pod, ask the API who you are, map
//      the network (recon-first-contact -> recon-who-am-i -> recon-neighborhood).
//   3. Identity/RBAC abuse (pod-exec -> create-pods -> sidecar-inject ->
//      rolebinding-escalate, the bind capstone).
//   4. Break out to the node (hostpath/privileged escape -> node-token-harvest,
//      the payoff -> static-pod-persist, staying below the API server).
//   5. Cluster-wide persistence (malicious-webhook).
// Update this list if Order changes. (The sidebar itself groups by Track, not by
// this flat order; this test guards the within-track Order that sorts each group.)
func TestCatalogFollowsCurriculumOrder(t *testing.T) {
	want := []string{"ns-escape", "suid-privesc", "ebpf-detect", "sa-impersonate", "recon-first-contact", "recon-who-am-i", "recon-neighborhood", "imagepullsecret-theft", "pod-exec", "create-pods", "sidecar-inject", "rolebinding-escalate", "hostpath-escape", "privileged-escape", "node-token-harvest", "ebpf-egress", "static-pod-persist", "malicious-webhook", "driver-research-dev", "driver-research-hard"}
	got := ListModules()
	if len(got) != len(want) {
		t.Fatalf("catalog has %d modules, want %d - update this test with the new module's place", len(got), len(want))
	}
	for i, slug := range want {
		if got[i].Slug != slug {
			t.Errorf("position %d = %q, want %q", i, got[i].Slug, slug)
		}
	}
}
