package topology

import (
	"context"
	"fmt"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Namespaces is the fixed set the map reports on - internal/health.Namespaces
// plus labs-runners (session runner pods), which health omits. Same
// "hardcoded for now" tradeoff called out there; a namespace whose List fails
// is skipped, not fatal (partial data beats none for a live dashboard).
var Namespaces = []string{
	"default",
	"labs",
	"labs-baseline",
	"labs-runners",
	"cert-manager",
	"vault",
	"argocd",
	"observability",
	"kube-system",
}

// Builder assembles the topology from the live cluster + the static wiring.
// Takes kubernetes.Interface (not the concrete *Clientset) so tests can inject
// a fake, same as labs.Manager. restConfig is kept for the dynamic client the
// health signals need (ArgoCD/cert-manager CRDs, see signals.go); nil in local
// dev / tests, where signals are simply omitted.
type Builder struct {
	clientset  kubernetes.Interface
	restConfig *rest.Config
}

func NewBuilder(clientset kubernetes.Interface, restConfig *rest.Config) *Builder {
	return &Builder{clientset: clientset, restConfig: restConfig}
}

// Build lists nodes and the pods in each of Namespaces, then staples on the
// static Edges/Layers/Dashboards. Sessions are attached by the caller (the API
// handler has the labs.Manager). Returns an error only when there's no
// clientset at all (local dev, not in-cluster); per-object List failures are
// swallowed so one missing namespace or a transient API hiccup can't blank the
// whole dashboard.
func (b *Builder) Build(ctx context.Context) (*Topology, error) {
	if b.clientset == nil {
		return nil, fmt.Errorf("topology unavailable: not running in-cluster")
	}
	t := &Topology{
		Edges:      Edges,
		Dashboards: Dashboards,
	}

	// The labs-node substrate depends on whether the kata-fc RuntimeClass is
	// actually installed (prod AX41) or not (local k3d runs lab pods as runc) -
	// derive it live so the map can't claim a microVM wall that isn't there.
	kata := false
	if rcs, err := b.clientset.NodeV1().RuntimeClasses().List(ctx, metav1.ListOptions{}); err == nil {
		for i := range rcs.Items {
			if rcs.Items[i].Name == "kata-fc" {
				kata = true
				break
			}
		}
	}
	t.Layers = LayersFor(kata)

	if nodes, err := b.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		t.Nodes = make([]Node, 0, len(nodes.Items))
		for _, n := range nodes.Items {
			t.Nodes = append(t.Nodes, summarizeNode(n))
		}
	}

	// Fan out across namespaces rather than walking them in sequence. Each
	// namespace costs six API round trips (pods + five resource counts), so
	// nine of them serially was ~54 round trips end to end with the API server
	// idle between each. They're independent reads, so latency is the only
	// thing being paid for - do them concurrently and the whole set costs about
	// as much as the slowest one.
	//
	// Results land in a fixed-size slice indexed by position, so the output
	// order stays the declaration order of Namespaces regardless of which
	// finishes first - the map's layout shouldn't shuffle between refreshes.
	entries := make([]*Namespace, len(Namespaces))
	var wg sync.WaitGroup
	for i, ns := range Namespaces {
		wg.Add(1)
		go func(i int, ns string) {
			defer wg.Done()
			pods, err := b.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				// Left nil and filtered out below - a namespace we can't read
				// (missing RBAC, or it doesn't exist in this environment)
				// shouldn't fail the whole map.
				return
			}
			entry := Namespace{Name: ns, Pods: make([]Pod, 0, len(pods.Items))}
			for _, p := range pods.Items {
				entry.Pods = append(entry.Pods, summarizePod(p))
			}
			// meta=nil: the outer cluster deliberately has no Secret/ConfigMap
			// read grant (see rbac.yaml), so those two kinds are omitted here.
			// The nested cluster passes a real meta client - it's the throwaway
			// target.
			entry.Resources = countResources(ctx, b.clientset, nil, ns)
			entries[i] = &entry
		}(i, ns)
	}

	// Health signals are independent of the namespace walk, so they overlap
	// with it instead of running after. Best-effort and swallowed on error - a
	// missing CRD or RBAC gap must not blank the map.
	var signals *Signals
	wg.Add(1)
	go func() {
		defer wg.Done()
		signals = buildSignals(ctx, b.restConfig)
	}()

	wg.Wait()

	t.Namespaces = make([]Namespace, 0, len(Namespaces))
	for _, e := range entries {
		if e != nil {
			t.Namespaces = append(t.Namespaces, *e)
		}
	}
	t.Signals = signals
	return t, nil
}

func summarizeNode(n corev1.Node) Node {
	ready := false
	for _, c := range n.Status.Conditions {
		if c.Type == corev1.NodeReady {
			ready = c.Status == corev1.ConditionTrue
			break
		}
	}
	return Node{
		Name:                 n.Name,
		Role:                 nodeRole(n),
		Ready:                ready,
		Kernel:               n.Status.NodeInfo.KernelVersion,
		Runtime:              n.Status.NodeInfo.ContainerRuntimeVersion,
		OS:                   n.Status.NodeInfo.OSImage,
		AllocatableCPUMillis: n.Status.Allocatable.Cpu().MilliValue(),
		AllocatableMemBytes:  n.Status.Allocatable.Memory().Value(),
	}
}

// nodeRole derives the region a node draws as. The labs worker carries
// k8labs.io/role=labs; everything
// with a control-plane role label is the control plane; else a plain worker.
func nodeRole(n corev1.Node) string {
	if _, ok := n.Labels["k8labs.io/role"]; ok {
		if n.Labels["k8labs.io/role"] == "labs" {
			return "labs"
		}
	}
	for k := range n.Labels {
		if k == "node-role.kubernetes.io/control-plane" || k == "node-role.kubernetes.io/master" {
			return "control-plane"
		}
	}
	return "worker"
}

func summarizePod(p corev1.Pod) Pod {
	var ready, total, restarts int32
	phase := string(p.Status.Phase)
	for _, cs := range p.Status.ContainerStatuses {
		total++
		if cs.Ready {
			ready++
		}
		restarts += cs.RestartCount
		// A container waiting-reason (CrashLoopBackOff, ImagePullBackOff) is
		// more useful than the pod phase, which stays "Running" while a
		// container inside crash-loops. Same call internal/health.summarize
		// makes.
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			phase = cs.State.Waiting.Reason
		}
	}
	containers := make([]string, 0, len(p.Spec.Containers))
	var memReq, cpuReq int64
	for _, c := range p.Spec.Containers {
		containers = append(containers, c.Name)
		// Scheduler-visible reservation: what this container REQUESTS, summed
		// across the pod. Used by the capacity summary to show the non-lab
		// baseline (platform pods) that competes with sessions for the node.
		// ResourceList's Cpu()/Memory() return a zero Quantity when unset, so a
		// requestless container simply adds nothing.
		cpuReq += c.Resources.Requests.Cpu().MilliValue()
		memReq += c.Resources.Requests.Memory().Value()
	}
	return Pod{
		Name:             p.Name,
		App:              p.Labels["app.kubernetes.io/name"],
		Workload:         workloadOf(p),
		Namespace:        p.Namespace,
		Node:             p.Spec.NodeName,
		Phase:            phase,
		Ready:            fmt.Sprintf("%d/%d", ready, total),
		Restarts:         restarts,
		IP:               p.Status.PodIP,
		Containers:       containers,
		MemRequestBytes:  memReq,
		CPURequestMillis: cpuReq,
	}
}

// workloadOf reduces a pod name to the stable workload key the static wiring
// references. Strips a StatefulSet ordinal (postgres-0 -> postgres) and up to
// two ReplicaSet/pod hash segments (backend-7c9d4-abcde -> backend,
// cilium-x8k2 -> cilium). Heuristic, but a dashboard label doesn't need
// ownerReference-grade precision, and it keeps the wiring free of generated
// names. Prefer the controller's name from ownerReferences later if this ever
// mis-groups something.
func workloadOf(p corev1.Pod) string {
	parts := strings.Split(p.Name, "-")
	// trailing StatefulSet ordinal
	if len(parts) > 1 && isAllDigits(parts[len(parts)-1]) {
		parts = parts[:len(parts)-1]
	}
	// up to two trailing hash segments (ReplicaSet hash + pod hash)
	for i := 0; i < 2 && len(parts) > 1; i++ {
		if isHashish(parts[len(parts)-1]) {
			parts = parts[:len(parts)-1]
		} else {
			break
		}
	}
	return strings.Join(parts, "-")
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isHashish matches the alnum suffixes Kubernetes appends (ReplicaSet
// template-hash + the pod's random suffix). Both are drawn from an alphabet
// that deliberately excludes vowels (bcdfghjklmnpqrstvwxz2456789) precisely so
// they can't spell words - so "no vowels" cleanly separates a hash segment
// from a real name ending like "server", "operator" or "controller", which
// the digit-presence test got wrong (a pod suffix can be all letters).
func isHashish(s string) bool {
	if len(s) < 4 || len(s) > 11 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
		case r == 'a' || r == 'e' || r == 'i' || r == 'o' || r == 'u':
			return false
		case r >= 'a' && r <= 'z':
		default:
			return false
		}
	}
	return true
}
