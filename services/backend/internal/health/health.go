// Package health reports pod status across the platform's cluster-level
// components, for the admin dashboard - see docs/architecture/README.md's
// System VM component list. Read-only, using the same in-cluster
// clientset internal/labs uses for session provisioning (see
// cmd/server/main.go), via the separate cluster-wide read-only RBAC grant
// apps/backend's own chart adds (apps/backend/templates/rbac.yaml) -
// distinct from labs' create/delete grant, which stays scoped to just the
// labs namespaces.
package health

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Namespaces is the fixed set the dashboard reports on - every namespace a
// component from this project's local-dev buildout actually landed in.
// Not (yet) environment-configurable - same "hardcoded for now" tradeoff
// as internal/labs/registry.go's module map.
var Namespaces = []string{
	"default",
	"labs",
	"labs-baseline",
	"cert-manager",
	"vault",
	"argocd",
	"observability",
	"kube-system",
}

type Pod struct {
	Name     string `json:"name"`
	Ready    string `json:"ready"`
	Phase    string `json:"phase"`
	Restarts int32  `json:"restarts"`
}

type Namespace struct {
	Name string `json:"name"`
	Pods []Pod  `json:"pods"`
}

type Checker struct {
	clientset *kubernetes.Clientset
}

func NewChecker(clientset *kubernetes.Clientset) *Checker {
	return &Checker{clientset: clientset}
}

// ClusterHealth lists pods in each of Namespaces. A namespace whose List
// call fails (doesn't exist, RBAC gap, API server hiccup) is skipped
// rather than failing the whole call - partial data beats none for a
// dashboard whose whole point is "what's actually up right now."
func (c *Checker) ClusterHealth(ctx context.Context) ([]Namespace, error) {
	if c.clientset == nil {
		return nil, fmt.Errorf("cluster health unavailable: not running in-cluster")
	}

	result := make([]Namespace, 0, len(Namespaces))
	for _, ns := range Namespaces {
		pods, err := c.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}
		entry := Namespace{Name: ns, Pods: make([]Pod, 0, len(pods.Items))}
		for _, p := range pods.Items {
			entry.Pods = append(entry.Pods, summarize(p))
		}
		result = append(result, entry)
	}
	return result, nil
}

func summarize(p corev1.Pod) Pod {
	var ready, total int32
	var restarts int32
	phase := string(p.Status.Phase)
	for _, cs := range p.Status.ContainerStatuses {
		total++
		if cs.Ready {
			ready++
		}
		restarts += cs.RestartCount
		if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
			// A container's waiting-reason (CrashLoopBackOff,
			// ImagePullBackOff, ...) is far more useful than the pod-level
			// phase, which stays "Running" even while a container inside
			// it is actively crash-looping.
			phase = cs.State.Waiting.Reason
		}
	}
	return Pod{
		Name:     p.Name,
		Ready:    fmt.Sprintf("%d/%d", ready, total),
		Phase:    phase,
		Restarts: restarts,
	}
}
