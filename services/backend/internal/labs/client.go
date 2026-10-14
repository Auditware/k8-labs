package labs

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// NewClientset builds an in-cluster client - relies on the ServiceAccount
// token/CA that Kubernetes automounts into this pod (backend's chart sets
// automountServiceAccountToken: true and grants it least-privilege RBAC in
// the labs namespace only, see apps/labs/templates/rbac.yaml). There is no
// out-of-cluster path: session provisioning only works when the backend
// itself is running inside the cluster it's managing.
//
// The *rest.Config is returned alongside the clientset because the
// nested-runner path needs it directly: fetching a session's nested
// kubeconfig is a pod exec (remotecommand, see runner.go), which builds its
// own SPDY executor from the REST config rather than going through the typed
// clientset.
func NewClientset() (*kubernetes.Clientset, *rest.Config, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, nil, err
	}

	// client-go defaults to QPS 5 / Burst 10, which is sized for a controller
	// making occasional calls, not for the topology dashboard - one Build()
	// issues ~58 requests (a pods list plus five resource counts across nine
	// namespaces, plus nodes/runtimeclasses/signals). At the default the burst
	// covers the first 10 and the remaining ~48 drain at 5/sec, so the endpoint
	// spent ~10s asleep in the client's own rate limiter with the API server
	// completely idle. Measured 3.8s / 5.5s / 11.2s for an 11KB response before
	// this change, degrading run-to-run as the limiter's bucket emptied.
	//
	// These are client-side limits only; the API server has its own
	// priority-and-fairness protection, which is the thing actually guarding
	// the control plane. Values chosen to comfortably cover one topology build
	// without queueing while still bounding a runaway loop.
	cfg.QPS = 50
	cfg.Burst = 100

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, nil, err
	}
	return cs, cfg, nil
}
