package topology

// Static wiring: the communication edges, host-substrate layers, and
// reverse-proxied dashboards that aren't cheaply discoverable from the
// Kubernetes API. Kept in Go (not a mounted ConfigMap) because it only
// changes when the platform's component set changes - already a code+image
// change - so a ConfigMap's edit-without-rebuild benefit rarely applies while
// its schema/parser/validation/drift cost is continuous.
//
// From/To are workload keys (pod.Workload, see builder.workloadOf), the
// synthetic "cloudflare" ingress source, or "session:<id>" injected by the API
// handler per active session. Ports document what the frontend annotates on
// each edge. Kinds: in (ingress), app (group-level), podapp (pod-specific),
// prov (provisioning), escape.

// Edges is the outer-cluster communication graph. Nested-cluster edges (the
// dev->production escape, runner->apiserver) are emitted per session in
// Phase 1b alongside the nested topology.
var Edges = []Edge{
	{From: "cloudflare", To: "traefik", Port: ":443", Kind: "in"},
	{From: "traefik", To: "frontend", Port: ":80", Kind: "app"},
	{From: "traefik", To: "backend", Port: ":8080", Kind: "app"},
	// The backend's busiest link: client-go to the cluster API for session
	// provisioning + topology/health reads. On kubeadm prod this anchors to the
	// kube-apiserver static pod; on k3s/k3d (API server runs in-process, no pod)
	// the frontend falls back to the kube-system hive. See topologyRenderer.
	{From: "backend", To: "kube-apiserver", Port: ":443", Kind: "podapp"},
	{From: "backend", To: "postgres", Port: ":5432", Kind: "podapp"},
	{From: "backend", To: "vault", Port: ":8200", Kind: "podapp"},
	{From: "argocd-server", To: "backend", Port: "", Kind: "app"},
	{From: "alloy", To: "loki", Port: ":3100", Kind: "podapp"},
	{From: "loki", To: "grafana", Port: ":3100", Kind: "podapp"},
	{From: "tetragon", To: "grafana", Port: "", Kind: "podapp"},
	// backend -> session:<id> (provision, :6443) is injected per session by the
	// API handler, since the target id isn't known until a session exists.
}

// LayersFor returns the host-substrate bands under each node region, keyed by
// Role (see builder.nodeRole). The kernel band's Sub is overridden by the live
// node's KernelVersion in the frontend. `kata` selects the real story of the
// labs node: with the kata-fc RuntimeClass present (prod AX41) it's the
// Firecracker microVM + devmapper + WireGuard stack; without it (local k3d,
// where Tilt only labels the agent role=labs for scheduling) it's plain runc
// in Docker - so the map can't claim a microVM wall the node doesn't have.
// This is the one place the static wiring is derived from a live fact instead
// of hardcoded, precisely because getting it wrong here would misrepresent the
// isolation guarantee.
func LayersFor(kata bool) []Layer {
	if kata {
		return []Layer{
			{NodeRole: "control-plane", Icon: "cni", Title: "Cilium CNI · WireGuard", Sub: "kube-proxy-replacement · cilium_wg0", Kind: "net"},
			{NodeRole: "control-plane", Icon: "containerd", Title: "containerd", Sub: "overlayfs"},
			{NodeRole: "control-plane", Icon: "kernel", Title: "kernel", Sub: "host · kubelet :10250"},
			{NodeRole: "labs", Icon: "kata", Title: "kata-fc · Firecracker", Sub: "RuntimeClass · microVM per lab pod", Kind: "wall"},
			{NodeRole: "labs", Icon: "containerd", Title: "containerd + devmapper", Sub: "block snapshotter · thin-pool"},
			{NodeRole: "labs", Icon: "cni", Title: "Cilium CNI · WireGuard", Sub: "cilium_wg0 peer"},
			{NodeRole: "labs", Icon: "kernel", Title: "kernel", Sub: "host · hostfw kubelet:10250 CP-only"},
		}
	}
	return []Layer{
		{NodeRole: "control-plane", Icon: "cni", Title: "Cilium CNI", Sub: "kube-proxy-replacement (single node)", Kind: "net"},
		{NodeRole: "control-plane", Icon: "containerd", Title: "containerd", Sub: "k3d in docker · overlayfs"},
		{NodeRole: "control-plane", Icon: "kernel", Title: "kernel", Sub: "host"},
		{NodeRole: "labs", Icon: "runc", Title: "runc · no microVM", Sub: "lab pods run as plain runc - dev only", Kind: "warn"},
		{NodeRole: "labs", Icon: "containerd", Title: "containerd (k3d in docker)", Sub: "overlayfs"},
		{NodeRole: "labs", Icon: "cni", Title: "Cilium CNI", Sub: "single node · no wireguard"},
		{NodeRole: "labs", Icon: "kernel", Title: "kernel", Sub: "host"},
	}
}

// Dashboards are the admin-gated reverse-proxied UIs (paths mirror the routes
// in internal/api/adminproxy.go and routes.go). Ref is a workload key or a
// namespace name the frontend anchors the link to.
var Dashboards = []Dashboard{
	{Ref: "grafana", Label: "Grafana", Path: "/admin/grafana"},
	{Ref: "argocd-server", Label: "ArgoCD", Path: "/admin/argocd"},
	{Ref: "vault", Label: "Vault UI", Path: "/ui"},
	{Ref: "backend", Label: "Swagger", Path: "/swagger/"},
}
