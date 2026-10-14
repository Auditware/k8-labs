package topology

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestWorkloadOf(t *testing.T) {
	// Hash segments use realistic vowel-free suffixes, matching the alphabet
	// Kubernetes actually draws them from.
	cases := map[string]string{
		"backend-7c9d4f8b6-2xq7v":         "backend",                       // Deployment: rs hash + pod suffix
		"frontend-5d4c9d7bf-x8k2z":        "frontend",                      // Deployment
		"postgres-0":                      "postgres",                      // StatefulSet ordinal
		"vault-0":                         "vault",                         // StatefulSet ordinal
		"cilium-x8k2z":                    "cilium",                        // DaemonSet: single pod suffix
		"cilium-operator-6b4c9d-2xq7v":    "cilium-operator",               // "operator" (vowels) kept
		"argocd-server-7f8d9b-2xq7v":      "argocd-server",                 // "server" (vowels) kept
		"argocd-application-controller-0": "argocd-application-controller", // ordinal only
		"coredns-668d6bf9bc-2xq7v":        "coredns",
		"traefik":                         "traefik", // bare name, nothing to strip
		"loki-0":                          "loki",
	}
	for in, want := range cases {
		p := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: in}}
		if got := workloadOf(p); got != want {
			t.Errorf("workloadOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEscapeEdges(t *testing.T) {
	// ns-escape shape: foothold pod in dev, flag Secret in production,
	// system namespaces with their own pods/secrets. Expect exactly one escape
	// edge: the foothold -> production, and nothing anchored to a system ns.
	nss := []Namespace{
		{Name: "kube-system", Pods: []Pod{{Name: "coredns", App: "coredns"}}, Resources: &NsResources{Secrets: 3}},
		{Name: "dev", Pods: []Pod{{Name: "foothold", App: "foothold"}}},
		{Name: "production", Pods: nil, Resources: &NsResources{Secrets: 1, SecretNames: []string{"flag"}}},
		{Name: "kube-public", Resources: &NsResources{Roles: 1}},
	}
	got := escapeEdges(nss)
	if len(got) != 1 {
		t.Fatalf("escapeEdges returned %d edges, want 1: %+v", len(got), got)
	}
	e := got[0]
	if e.From != "foothold" || e.To != "secret:production" || e.Kind != "escape" {
		t.Errorf("escape edge = %+v, want foothold -> secret:production (escape)", e)
	}

	// No foothold (no non-system pods) -> no escape edge derivable.
	if got := escapeEdges([]Namespace{{Name: "production", Resources: &NsResources{Secrets: 1}}}); got != nil {
		t.Errorf("escapeEdges with no source pod = %+v, want nil", got)
	}
}
