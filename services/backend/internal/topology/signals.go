package topology

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// Signals are the API-derivable health rollups the map overlays. devmapper
// utilization and WireGuard peer state are deliberately absent - they're host/
// Cilium facts the Kubernetes API doesn't surface, so they need a node-metrics
// or cilium-status integration (a separate lift), not a CRD read.
type Signals struct {
	Argo  *ArgoSignal `json:"argo,omitempty"`
	Certs *CertSignal `json:"certs,omitempty"`
}

// ArgoSignal is the app-of-apps sync rollup: how many ArgoCD Applications are
// Synced vs. drifted, and which ones drifted.
type ArgoSignal struct {
	Synced    int      `json:"synced"`
	Total     int      `json:"total"`
	OutOfSync []string `json:"outOfSync,omitempty"`
}

// CertSignal is the soonest cert-manager Certificate expiry (in days) plus any
// expiring within the warning window - so an about-to-lapse origin/ingress cert
// surfaces before it breaks TLS.
type CertSignal struct {
	SoonestDays int      `json:"soonestDays"`
	Expiring    []string `json:"expiring,omitempty"` // ns/name, expiring < certWarnDays
}

const certWarnDays = 14

var (
	argoGVR = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	certGVR = schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"}
)

// buildSignals reads the ArgoCD Application and cert-manager Certificate CRDs
// via a dynamic client. Bounded by a short timeout; every failure path returns
// what it has (or nil) rather than erroring - these are an overlay, never a
// reason to fail the whole topology. nil restConfig (local dev / tests) -> nil.
func buildSignals(ctx context.Context, cfg *rest.Config) *Signals {
	if cfg == nil {
		return nil
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	s := &Signals{}
	if apps, err := dyn.Resource(argoGVR).List(ctx, metav1.ListOptions{}); err == nil {
		a := &ArgoSignal{Total: len(apps.Items)}
		for i := range apps.Items {
			status, _, _ := unstructured.NestedString(apps.Items[i].Object, "status", "sync", "status")
			if status == "Synced" {
				a.Synced++
			} else {
				a.OutOfSync = append(a.OutOfSync, apps.Items[i].GetName())
			}
		}
		s.Argo = a
	}
	if certs, err := dyn.Resource(certGVR).List(ctx, metav1.ListOptions{}); err == nil {
		c := &CertSignal{SoonestDays: -1}
		now := time.Now()
		for i := range certs.Items {
			notAfter, ok, _ := unstructured.NestedString(certs.Items[i].Object, "status", "notAfter")
			if !ok || notAfter == "" {
				continue
			}
			exp, perr := time.Parse(time.RFC3339, notAfter)
			if perr != nil {
				continue
			}
			days := int(exp.Sub(now).Hours() / 24)
			if c.SoonestDays < 0 || days < c.SoonestDays {
				c.SoonestDays = days
			}
			if days < certWarnDays {
				c.Expiring = append(c.Expiring, certs.Items[i].GetNamespace()+"/"+certs.Items[i].GetName())
			}
		}
		if len(certs.Items) > 0 {
			s.Certs = c
		}
	}
	if s.Argo == nil && s.Certs == nil {
		return nil
	}
	return s
}
