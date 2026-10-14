package topology

import (
	"context"
)

// The student-facing topology.
//
// This is a SEPARATE BUILDER, not a filtered admin payload, and that is the
// whole security design. Build() reaches into the real cluster and returns node
// names, kernel and runtime versions, every namespace's pods and their IPs,
// secret names, the reverse-proxy paths to ArgoCD/Grafana/Vault, ArgoCD sync
// state, cert expiry, and every user's session id. Filtering that down for a
// student would put one forgotten field between a lesson and an infrastructure
// leak, forever, on every future change to the admin payload.
//
// Scoped() instead reads exclusively from the student's own nested cluster -
// the disposable k3s their session booted. The outer cluster's data never
// enters this code path, so there is nothing to remember to strip. Adding a
// field to the admin topology cannot leak here, because this function cannot
// see it.
//
// What a student is shown:
//   - their own nested cluster's namespaces and pods, which is the lab itself;
//   - one opaque boundary node standing for everything outside it.
//
// What is deliberately absent, and must stay absent:
//   - Nodes: real node names plus KernelVersion/ContainerRuntime/OSImage. A
//     nested node would name the microVM's kernel, which is the containment
//     boundary the labs are built on - not something to hand an attacker.
//   - Layers: the host substrate (kernel, runtime, firewall bands).
//   - Dashboards: /admin/argocd, /admin/grafana, Vault. These reveal the
//     internal stack even though the routes themselves are admin-gated.
//   - Signals: ArgoCD drift and cert expiry.
//   - Sessions: any session, including their own id - the caller already knows
//     it, and the field is shaped to carry other people's.
//   - escapeEdges: the derived foothold -> production "get secret" edge, which
//     is literally the answer to ns-escape. See Scoped's edge handling.

// BoundaryRef is the id of the opaque node representing "everything outside
// your lab". Deliberately not a real workload: the frontend renders it as a
// featureless platform marker with no name, version or address.
const BoundaryRef = "k8labs-platform"

// Scoped builds the topology a student may see for one of their own sessions,
// from that session's nested kubeconfig and nothing else.
//
// spoilers=false drops the derived escape edge. That edge is the lab's answer
// drawn as a picture, so it is withheld until the student has solved it; the
// caller decides, because only it knows whether they have.
func Scoped(ctx context.Context, kubeconfig []byte, spoilers bool) *ScopedTopology {
	out := &ScopedTopology{Boundary: BoundaryRef}

	cluster := NestedFromKubeconfig(ctx, kubeconfig)
	if cluster == nil || !cluster.Reachable {
		// Reachable=false is normal, not an error: the session may still be
		// provisioning, or have just been torn down. Say so plainly rather than
		// rendering an empty diagram that looks like a broken cluster.
		out.Error = "your lab cluster isn't reachable yet"
		if cluster != nil && cluster.Error != "" {
			out.Error = cluster.Error
		}
		return out
	}

	// Show only the lab's own namespaces. k3s's plumbing - kube-system
	// (coredns, local-path-provisioner), kube-public, kube-node-lease, and an
	// empty default - is not the lab, and its secret/config counts render as
	// amber noise that reads as "something's unhealthy". A student should see
	// the cluster they attack, not the machinery underneath it.
	for _, ns := range cluster.Namespaces {
		if isSystemNamespace(ns.Name) {
			continue
		}
		out.Namespaces = append(out.Namespaces, ns)
	}
	if spoilers {
		out.Edges = cluster.Edges
	}
	// No synthetic platform->cluster edge here. There used to be one, and it
	// never drew: its endpoints were abstract ids the renderer has no way to
	// resolve, so it silently dropped and the boundary node floated
	// disconnected. The join between the boundary and the student's cluster is
	// structural - the renderer draws it from the regions themselves - so this
	// list stays what it claims to be: edges inside the student's own cluster.
	return out
}

// ScopedTopology is the whole payload GET /api/v1/sessions/{id}/topology
// returns. Its field set is the contract: it has no place to put a node name, a
// kernel version, a dashboard path, a signal, or another user's session, so a
// future edit cannot accidentally include one without someone adding a field
// and being asked why.
type ScopedTopology struct {
	// Boundary is the id of the opaque outside-your-lab node.
	Boundary string `json:"boundary"`
	// Namespaces are the student's own nested cluster's, including pod names
	// and container lists. Safe: this cluster is the disposable target, created
	// for them and destroyed with their session.
	Namespaces []Namespace `json:"namespaces,omitempty"`
	Edges      []Edge      `json:"edges,omitempty"`
	// Error explains an unreachable cluster in the student's terms.
	Error string `json:"error,omitempty"`
}
