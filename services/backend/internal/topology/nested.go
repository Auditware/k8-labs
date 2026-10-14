package topology

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/tools/clientcmd"
)

// NestedCluster is a lab session's disposable nested k3s cluster, enumerated
// live from the kubeconfig the session caches. Reachable=false with Error set
// when the nested API can't be reached (torn down, cache stale, network) - the
// outer session still renders on the map, just without its interior.
type NestedCluster struct {
	Reachable  bool        `json:"reachable"`
	Namespaces []Namespace `json:"namespaces,omitempty"`
	// Edges is the nested cluster's internal communication graph - currently
	// the lab's cross-namespace escape path (foothold -> the Secret it was never
	// meant to reach), derived from the enumerated contents. See escapeEdges.
	Edges []Edge `json:"edges,omitempty"`
	Error string `json:"error,omitempty"`
}

// escapeEdges derives the lab's cross-namespace escape link(s): from the
// foothold/entry pod to every non-system namespace holding a Secret it was
// never meant to reach. For ns-escape that's foothold(dev) -> production
// (the flag Secret) - the attack path is the whole point of the lab, so the map
// should draw it. Heuristic but honest: source = the first pod in a
// non-system namespace, targets = other non-system namespaces with a Secret.
// isSystemNamespace reports whether a namespace is cluster plumbing rather than
// lab content - k3s's own namespaces plus an (assumed-empty) default. Used to
// keep both the escape-edge heuristic and the student topology focused on the
// lab, not the machinery.
func isSystemNamespace(name string) bool {
	switch name {
	case "kube-system", "kube-public", "kube-node-lease", "default":
		return true
	}
	return false
}

func escapeEdges(nss []Namespace) []Edge {
	var srcKey, srcNS string
	for _, ns := range nss {
		if isSystemNamespace(ns.Name) || len(ns.Pods) == 0 {
			continue
		}
		p := ns.Pods[0]
		if srcKey = p.App; srcKey == "" {
			srcKey = p.Name
		}
		srcNS = ns.Name
		break
	}
	if srcKey == "" {
		return nil
	}
	var edges []Edge
	for _, ns := range nss {
		if isSystemNamespace(ns.Name) || ns.Name == srcNS || ns.Resources == nil || ns.Resources.Secrets == 0 {
			continue
		}
		// Target the secret itself, not the namespace: the attack reads a
		// specific Secret, so the arrow should land on the secret hex the
		// renderer draws inside the hive, not on the hive outline. See the
		// "secret:" resolver in topologyRenderer.ts.
		edges = append(edges, Edge{From: srcKey, To: "secret:" + ns.Name, Port: "get secret", Kind: "escape"})
	}
	return edges
}

// NestedFromKubeconfig dials the nested cluster directly - its kubeconfig's
// server is already the runner pod's IP (rewritten by the runner entrypoint,
// see labs/nested.go), so this reaches the nested API without proxying through
// the outer cluster - and lists every namespace's pods. Bounded by a short
// timeout so a dead or slow nested API can't stall the topology endpoint.
// Never returns nil.
func NestedFromKubeconfig(ctx context.Context, kubeconfig []byte) *NestedCluster {
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return &NestedCluster{Error: "parse kubeconfig: " + err.Error()}
	}
	cfg.Timeout = 4 * time.Second
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return &NestedCluster{Error: "nested client: " + err.Error()}
	}
	// Metadata client for the nested cluster's Secret/ConfigMap names (never
	// their values). The nested cluster is the disposable throwaway target, so
	// reading its object metadata carries none of the outer cluster's risk.
	nestedMeta, _ := metadata.NewForConfig(cfg)
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	nss, err := cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return &NestedCluster{Error: err.Error()}
	}
	nc := &NestedCluster{Reachable: true, Namespaces: make([]Namespace, 0, len(nss.Items))}
	for _, ns := range nss.Items {
		pods, err := cs.CoreV1().Pods(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		entry := Namespace{Name: ns.Name, Pods: make([]Pod, 0, len(pods.Items))}
		for _, p := range pods.Items {
			entry.Pods = append(entry.Pods, summarizePod(p))
		}
		entry.Resources = countResources(ctx, cs, nestedMeta, ns.Name)
		nc.Namespaces = append(nc.Namespaces, entry)
	}
	nc.Edges = escapeEdges(nc.Namespaces)
	return nc
}
