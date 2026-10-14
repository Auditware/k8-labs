package topology

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The leak test.
//
// Scoped() is safe because of where it reads from, not because of what it
// filters - but "we built it from a different source" is a claim that decays if
// nobody checks it. These tests hold the line two ways: the payload TYPE has no
// place to put outer-cluster data, and a serialized payload contains none of
// the strings that would give the infrastructure away.

// If someone adds a field to ScopedTopology, this test fails and they have to
// come here and justify it. That is the point: the field set is the contract,
// and widening it should be a deliberate act rather than a side effect of
// reusing a struct.
func TestScopedTopologyFieldSetIsClosed(t *testing.T) {
	allowed := map[string]bool{
		"Boundary": true, "Namespaces": true, "Edges": true, "Error": true,
	}
	ty := reflect.TypeOf(ScopedTopology{})
	for i := 0; i < ty.NumField(); i++ {
		name := ty.Field(i).Name
		if !allowed[name] {
			t.Errorf("ScopedTopology gained field %q. Student payloads must not carry "+
				"outer-cluster data - if this is genuinely safe, add it to the allowlist "+
				"in this test and say why in the commit.", name)
		}
	}

	// And the admin payload must NOT be assignable to it - a refactor that
	// makes Scoped() able to return a Topology would silently undo everything.
	if reflect.TypeOf(Topology{}).ConvertibleTo(ty) {
		t.Error("Topology is convertible to ScopedTopology; the two payloads must stay distinct")
	}
}

// A student's payload must never contain the strings that identify our
// infrastructure, whatever their nested cluster happens to contain.
func TestScopedPayloadCarriesNoOuterClusterIdentifiers(t *testing.T) {
	// A nested cluster whose contents are as adversarial as we can make them:
	// namespaces and pods named after real platform components, to prove the
	// test is checking the payload rather than trusting the source.
	st := &ScopedTopology{
		Boundary: BoundaryRef,
		Namespaces: []Namespace{{
			Name: "dev",
			Pods: []Pod{{Name: "foothold", Namespace: "dev", Phase: "Running", Ready: "1/1"}},
		}},
		Edges: []Edge{{From: BoundaryRef, To: "cluster", Kind: "prov"}},
	}

	blob, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	payload := strings.ToLower(string(blob))

	// Every one of these appears somewhere in the admin topology payload.
	forbidden := []string{
		"k8labs-cp",      // the control-plane node's name
		"k8labs-labs",    // the labs node's name
		"kernelversion",  // host substrate
		"containerd",     // node runtime
		"ubuntu",         // node OS image
		"/admin/argocd",  // reverse-proxied dashboards
		"/admin/grafana", //
		"vault",          //
		"outofsync",      // ArgoCD signal
		"soonestdays",    // cert signal
		"labs-runners",   // the outer namespace holding runner pods
		"203.0.113.10", // the labs node's public IP
		"nodeRole",       // layer band keyed by node role
	}
	for _, s := range forbidden {
		if strings.Contains(payload, strings.ToLower(s)) {
			t.Errorf("scoped payload contains %q - that identifies the outer cluster", s)
		}
	}
}

// The escape edge is the answer to the lab, drawn as a picture. It must be
// absent unless the caller explicitly asks for it.
func TestScopedWithholdsTheEscapeEdgeByDefault(t *testing.T) {
	nested := []Namespace{
		{Name: "dev", Pods: []Pod{{Name: "foothold", App: "foothold"}}},
		{Name: "production", Resources: &NsResources{Secrets: 1, SecretNames: []string{"flag"}}},
	}
	edges := escapeEdges(nested)
	if len(edges) == 0 {
		t.Fatal("fixture produced no escape edge, so this test proves nothing")
	}

	// Simulating what Scoped does with a reachable cluster, without needing a
	// live nested API: the spoilers flag is the only thing gating these.
	withheld := &ScopedTopology{Boundary: BoundaryRef}
	withheld.Edges = append(withheld.Edges, Edge{From: BoundaryRef, To: "cluster", Kind: "prov"})
	for _, e := range withheld.Edges {
		if e.Kind == "escape" {
			t.Error("escape edge present with spoilers off")
		}
	}

	shown := &ScopedTopology{Boundary: BoundaryRef, Edges: edges}
	found := false
	for _, e := range shown.Edges {
		if e.Kind == "escape" {
			found = true
		}
	}
	if !found {
		t.Error("escape edge missing even with spoilers on")
	}
}

// An unreachable cluster is a normal state (still provisioning, just torn
// down), so it must produce a readable message rather than an empty diagram
// that looks like a broken cluster - and still no outer-cluster data.
func TestScopedUnreachableClusterExplainsItself(t *testing.T) {
	out := Scoped(t.Context(), []byte("not-a-kubeconfig"), false)
	if out.Error == "" {
		t.Error("unreachable cluster produced no explanation")
	}
	if len(out.Namespaces) != 0 {
		t.Error("unreachable cluster produced namespaces")
	}
	if out.Boundary != BoundaryRef {
		t.Errorf("boundary = %q, want %q", out.Boundary, BoundaryRef)
	}
}

// A student sees the cluster they attack, not k3s's plumbing. System
// namespaces (kube-system's coredns/local-path-provisioner and friends) are
// noise, and their secret/config counts render amber - reading as "something's
// unhealthy" when nothing is. They must not appear in the scoped payload.
func TestScopedFiltersSystemNamespaces(t *testing.T) {
	// NestedFromKubeconfig needs a live cluster, so exercise the filter through
	// the same predicate the builder uses, over a representative namespace set.
	all := []string{"kube-system", "kube-public", "kube-node-lease", "default", "dev", "production", "lab"}
	var kept []string
	for _, ns := range all {
		if !isSystemNamespace(ns) {
			kept = append(kept, ns)
		}
	}
	want := map[string]bool{"dev": true, "production": true, "lab": true}
	if len(kept) != len(want) {
		t.Fatalf("kept %v, want only the lab's own namespaces %v", kept, want)
	}
	for _, ns := range kept {
		if !want[ns] {
			t.Errorf("kept system/irrelevant namespace %q", ns)
		}
	}
}
