package topology

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/metadata"
)

var (
	secretsGVR    = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}
	configMapsGVR = schema.GroupVersionResource{Version: "v1", Resource: "configmaps"}
)

// countResources tallies the security-relevant namespaced objects for one
// namespace. Reused for both the outer cluster (Builder's clients) and a
// session's nested cluster (clients built from its kubeconfig).
//
// Secrets and ConfigMaps are counted ONLY when a metadata client is passed,
// and even then only their names/counts are read - never their contents. The
// outer cluster passes meta=nil on purpose: it has no `list secrets` grant
// (that verb has no metadata-only form, so granting it would authorize reading
// every Secret value cluster-wide - too much for an internet-facing SA). Only
// a session's NESTED throwaway cluster passes a real meta client, reached
// through that session's own kubeconfig - that's where the lab flag Secret is.
//
// Every list is best-effort: a denied/failing kind is left at zero rather than
// failing the whole rollup. Returns nil when nothing was found (so the JSON
// omits an empty object).
func countResources(ctx context.Context, cs kubernetes.Interface, meta metadata.Interface, ns string) *NsResources {
	r := &NsResources{}
	if x, err := cs.CoreV1().Services(ns).List(ctx, metav1.ListOptions{}); err == nil {
		r.Services = len(x.Items)
	}
	if x, err := cs.CoreV1().ServiceAccounts(ns).List(ctx, metav1.ListOptions{}); err == nil {
		r.ServiceAccounts = len(x.Items)
	}
	if x, err := cs.RbacV1().Roles(ns).List(ctx, metav1.ListOptions{}); err == nil {
		r.Roles = len(x.Items)
	}
	if x, err := cs.RbacV1().RoleBindings(ns).List(ctx, metav1.ListOptions{}); err == nil {
		r.RoleBindings = len(x.Items)
	}
	if x, err := cs.NetworkingV1().NetworkPolicies(ns).List(ctx, metav1.ListOptions{}); err == nil {
		r.NetworkPolicies = len(x.Items)
	}
	if meta != nil {
		if x, err := meta.Resource(secretsGVR).Namespace(ns).List(ctx, metav1.ListOptions{}); err == nil {
			r.Secrets = len(x.Items)
			for i := range x.Items {
				if len(r.SecretNames) >= 12 {
					break
				}
				r.SecretNames = append(r.SecretNames, x.Items[i].GetName())
			}
		}
		if x, err := meta.Resource(configMapsGVR).Namespace(ns).List(ctx, metav1.ListOptions{}); err == nil {
			r.ConfigMaps = len(x.Items)
		}
	}
	if r.Secrets == 0 && r.ConfigMaps == 0 && r.Services == 0 && r.ServiceAccounts == 0 &&
		r.Roles == 0 && r.RoleBindings == 0 && r.NetworkPolicies == 0 {
		return nil
	}
	return r
}
