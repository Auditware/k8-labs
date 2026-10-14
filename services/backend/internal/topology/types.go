// Package topology builds the admin dashboard's live cluster map: nodes ->
// namespaces -> pods -> containers, joined with the static wiring
// (communication edges, host-substrate layers, reverse-proxied dashboards)
// from wiring.go. Read-only, over the same cluster-wide read-only clientset
// internal/health uses (see apps/backend/templates/rbac.yaml - this package
// adds `nodes` to that grant). The static wiring lives in Go rather than a
// ConfigMap on purpose.
package topology

// Node is a cluster node as the map draws it - a region containing its
// namespaces' pods, with the host-substrate facts (kernel/runtime) that the
// static Layers can't know per-machine.
type Node struct {
	Name    string `json:"name"`
	Role    string `json:"role"` // "control-plane" | "labs" | "worker"
	Ready   bool   `json:"ready"`
	Kernel  string `json:"kernel"`  // from Status.NodeInfo.KernelVersion
	Runtime string `json:"runtime"` // from Status.NodeInfo.ContainerRuntimeVersion
	OS      string `json:"os"`      // from Status.NodeInfo.OSImage
	// Allocatable is what the scheduler may hand out on this node (Status.
	// Allocatable, i.e. capacity minus system/kube reservations). It is the
	// honest denominator for the capacity summary: sessions are scheduled
	// against allocatable, not raw hardware.
	AllocatableCPUMillis int64 `json:"allocatableCpuMillis"`
	AllocatableMemBytes  int64 `json:"allocatableMemBytes"`
}

// Pod carries enough to render a mini-hex and its container list, plus the
// node it runs on (so the frontend groups pods under the right node region)
// and a workload key the static edges/dashboards reference (see workloadOf).
type Pod struct {
	Name string `json:"name"`
	// App is the pod's app.kubernetes.io/name label - the stable key the
	// static wiring's edges/dashboards match on (a Helm release can name the
	// workload <release>-<chart>, e.g. backend-backend, so the name-derived
	// Workload isn't reliable for matching; the label is). Empty if unlabelled;
	// the frontend then falls back to Workload/name.
	App        string   `json:"app,omitempty"`
	Workload   string   `json:"workload"`
	Namespace  string   `json:"namespace"`
	Node       string   `json:"node,omitempty"`
	Phase      string   `json:"phase"`
	Ready      string   `json:"ready"`
	Restarts   int32    `json:"restarts"`
	IP         string   `json:"ip,omitempty"`
	Containers []string `json:"containers"`
	// Scheduler-visible resource requests summed over the pod's containers. The
	// capacity summary sums these across non-session pods on the labs node to
	// draw the platform baseline (see Capacity.MemSystemReservedBytes).
	MemRequestBytes  int64 `json:"memRequestBytes,omitempty"`
	CPURequestMillis int64 `json:"cpuRequestMillis,omitempty"`
}

// Namespace is a hive: a namespace and the pods inside it, plus a rollup of
// the security-relevant non-pod objects (Resources) for the popover inventory.
type Namespace struct {
	Name      string       `json:"name"`
	Pods      []Pod        `json:"pods"`
	Resources *NsResources `json:"resources,omitempty"`
}

// NsResources counts the security-relevant namespaced objects. Secrets and
// ConfigMaps are read metadata-only (names/counts, never values) - see
// countResources. SecretNames lets the map show e.g. the flag Secret without
// ever reading its data.
type NsResources struct {
	Secrets         int      `json:"secrets,omitempty"`
	ConfigMaps      int      `json:"configMaps,omitempty"`
	Services        int      `json:"services,omitempty"`
	ServiceAccounts int      `json:"serviceAccounts,omitempty"`
	Roles           int      `json:"roles,omitempty"`
	RoleBindings    int      `json:"roleBindings,omitempty"`
	NetworkPolicies int      `json:"networkPolicies,omitempty"`
	SecretNames     []string `json:"secretNames,omitempty"`
}

// Edge is one directed communication link. From/To are workload keys (see
// wiring.go), the synthetic "cloudflare" ingress source, or "session:<id>"
// for a lab session. Kind drives the frontend's styling: in (ingress), app
// (group-level), podapp (pod-specific), prov (provisioning), escape.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Port string `json:"port,omitempty"`
	Kind string `json:"kind"`
}

// Layer is one host-substrate band under a node region, keyed by node Role.
type Layer struct {
	NodeRole string `json:"nodeRole"`
	Icon     string `json:"icon,omitempty"`
	Title    string `json:"title"`
	Sub      string `json:"sub,omitempty"`
	Kind     string `json:"kind,omitempty"` // "net" | "wall" | "warn"
}

// Dashboard is a reverse-proxied UI link anchored to a workload or namespace.
// Path mirrors the routes in internal/api/adminproxy.go.
type Dashboard struct {
	Ref   string `json:"ref"`
	Label string `json:"label"`
	Path  string `json:"path"`
}

// Session is an active lab session. Cluster is the enumerated nested k3s
// cluster when its kubeconfig is cached and the nested API is reachable at poll
// time (see NestedFromKubeconfig); nil otherwise. Every session is nested-shape
// - see docs/architecture/lab-session-rewrite.md, F1.
type Session struct {
	ID     string `json:"id"`
	Module string `json:"module"`
	Status string `json:"status"`
	// ReservedBytes is the memory this session holds on the labs node
	// (labs.SessionMemory), and GiBMinutes is what it has accrued so far
	// (reservation x minutes alive). Both are the live cost, set by the topology
	// handler from the session manager - the admin map is where an operator wants
	// to see what each running lab is actually costing, not just that it exists.
	ReservedBytes int64          `json:"reservedBytes,omitempty"`
	GiBMinutes    float64        `json:"gibMinutes,omitempty"`
	Cluster       *NestedCluster `json:"cluster,omitempty"`
}

// Capacity is the labs-node resource rollup the admin summary charts. All four
// numbers are what the SCHEDULER sees, not live host telemetry: reservations
// (what sessions request) against allocatable (what the node offers) and the
// backend's own admission budget. This is deliberate - actual host CPU/memory
// needs node_exporter (the kubelet stats API 500s whenever a Kata pod runs), so
// the honest, always-available picture is the reservation one, which is also the
// number that actually decides whether the next session fits.
//
// Assembled by the admin topology handler, which is the one place that has both
// the live sessions (their cost) and the nodes (their allocatable).
type Capacity struct {
	// Memory, in bytes. Reserved is the sum of live sessions' SessionMemory;
	// Budget is the admission gate's ceiling (0 = disabled); Allocatable is the
	// labs node(s) total. Budget usually sits below Allocatable on purpose - the
	// backend refuses just before the scheduler would.
	MemReservedBytes    int64 `json:"memReservedBytes"`
	MemBudgetBytes      int64 `json:"memBudgetBytes"`
	MemAllocatableBytes int64 `json:"memAllocatableBytes"`
	// SystemReserved is the non-lab baseline: the summed requests of the platform
	// pods scheduled on the labs node(s) - DaemonSets (tetragon, CNI), coredns and
	// the like - which hold real allocatable before any session starts. Shown as a
	// muted base under the lab usage so a column reads as honest headroom rather
	// than empty. Excludes session-runner pods, which are the MemReserved above.
	MemSystemReservedBytes  int64 `json:"memSystemReservedBytes"`
	CPUSystemReservedMillis int64 `json:"cpuSystemReservedMillis"`
	// CPU, in millicores. Reserved is the sum of live sessions' CPU requests;
	// Allocatable is the labs node(s) total. There is no "used" here by design -
	// see the type comment.
	CPUReservedMillis    int64 `json:"cpuReservedMillis"`
	CPUAllocatableMillis int64 `json:"cpuAllocatableMillis"`
	// ByUser breaks the reserved totals down by the user whose sessions hold
	// them, so the admin charts can attribute consumption (who is using how much)
	// rather than only showing the aggregate. Every reserved byte belongs to some
	// user's session - there is no unattributed platform overhead in these
	// numbers - so the parts sum to the totals above. Sorted heaviest-first and
	// capped: past a handful of users the tail folds into a single "others" row
	// (User == ""), keeping the stacked column readable.
	ByUser []CapacityUser `json:"byUser,omitempty"`
}

// CapacityUser is one user's slice of the reserved totals (see Capacity.ByUser).
type CapacityUser struct {
	User      string `json:"user"` // username, or "" for the folded "others" row
	MemBytes  int64  `json:"memBytes"`
	CPUMillis int64  `json:"cpuMillis"`
	Sessions  int    `json:"sessions"`
}

// Topology is the whole payload GET /api/v1/admin/topology returns.
type Topology struct {
	Nodes      []Node      `json:"nodes"`
	Namespaces []Namespace `json:"namespaces"`
	Edges      []Edge      `json:"edges"`
	Layers     []Layer     `json:"layers"`
	Dashboards []Dashboard `json:"dashboards"`
	Sessions   []Session   `json:"sessions"`
	Signals    *Signals    `json:"signals,omitempty"`
	Capacity   *Capacity   `json:"capacity,omitempty"`
}
