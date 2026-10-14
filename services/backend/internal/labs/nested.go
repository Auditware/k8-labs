package labs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

// nestedKubeconfigPath is where the session-runner entrypoint publishes the
// nested cluster's kubeconfig (server address already rewritten to the pod IP).
// See services/session-runner/entrypoint.sh.
const nestedKubeconfigPath = "/output/kubeconfig"

// nestedReadyPoll is how often we re-check a nested readiness gate (node Ready,
// foothold pod Ready). Each gate returns the instant it sees the condition, so
// the poll interval is pure added latency - on average half of it per gate. At
// 2s that was ~1s wasted on the foothold gate alone, which dominates click-to-
// running (a cold image pull into the nested cluster). 500ms matches waitForReady
// (the runner-pod gate) and trims ~1.5s off a typical boot; the extra single-
// object GETs against one freshly-booted nested API server are negligible.
const nestedReadyPoll = 500 * time.Millisecond

// fetchNestedKubeconfig reads the nested cluster's kubeconfig out of the runner
// pod by exec-ing `cat /output/kubeconfig` (remotecommand/SPDY over the outer
// API). This is the deliberate handoff from A2/A3: the runner needs no outer
// RBAC to publish anything - the backend, which already has pod/exec rights,
// pulls the file. The entrypoint writes the file only after k3s comes up, so we
// retry until it's present and non-empty (or ctx expires).
func (m *Manager) fetchNestedKubeconfig(ctx context.Context, id string) ([]byte, error) {
	if m.restConfig == nil {
		return nil, fmt.Errorf("no rest config: nested provisioning needs in-cluster access")
	}
	name := runnerName(id)
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	for {
		out, err := m.execInRunner(ctx, name, []string{"cat", nestedKubeconfigPath})
		if err == nil && len(bytes.TrimSpace(out)) > 0 {
			return out, nil
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out fetching nested kubeconfig for %s: %w", id, ctx.Err())
		case <-time.After(1 * time.Second):
		}
	}
}

// execInRunner runs a command in the runner pod's single container and returns
// its stdout. Thin wrapper over remotecommand so fetchNestedKubeconfig (and any
// future exec need) don't each rebuild the SPDY plumbing.
func (m *Manager) execInRunner(ctx context.Context, podName string, command []string) ([]byte, error) {
	req := m.clientset.CoreV1().RESTClient().Post().
		Resource("pods").Name(podName).Namespace(m.cfg.RunnerNamespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: "runner",
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(m.restConfig, "POST", req.URL())
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	if err := exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr}); err != nil {
		return nil, fmt.Errorf("exec %v: %w (stderr: %s)", command, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// nestedRESTConfig builds a REST config for the nested cluster from its fetched
// kubeconfig. The kubeconfig's server is already the runner pod's IP (rewritten
// by the entrypoint) and its CA is valid for that address (--tls-san), so this
// dials the nested API directly - no proxying through the outer cluster.
func nestedRESTConfig(kubeconfig []byte) (*rest.Config, error) {
	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("parse nested kubeconfig: %w", err)
	}
	return cfg, nil
}

// waitForNestedReady blocks until the nested cluster's own node reports Ready -
// i.e. its kubelet is up and can schedule the lab pods B3 is about to apply.
// The runner pod being Ready only means the API is listening (TCP 6443); this
// is the stronger signal that the cluster can actually run workloads.
func waitForNestedReady(ctx context.Context, cfg *rest.Config) error {
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("nested clientset: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	for {
		nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err == nil {
			for _, n := range nodes.Items {
				for _, c := range n.Status.Conditions {
					if c.Type == corev1.NodeReady && c.Status == corev1.ConditionTrue {
						return nil
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for nested node Ready: %w", ctx.Err())
		case <-time.After(nestedReadyPoll):
		}
	}
}

// waitForNestedPodReady blocks until a specific pod inside the nested cluster
// is Ready - used for the module's entry/foothold pod so a session isn't
// reported "running" (and the terminal isn't offered) until the pod the
// terminal execs into actually exists and is up. Without this, the first
// terminal connect races the foothold image pull and gets an empty shell.
func waitForNestedPodReady(ctx context.Context, cfg *rest.Config, namespace, pod string) error {
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("nested clientset: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	for {
		p, err := cs.CoreV1().Pods(namespace).Get(ctx, pod, metav1.GetOptions{})
		if err == nil {
			if p.Status.Phase == corev1.PodFailed {
				return fmt.Errorf("nested entry pod %s/%s failed: %s", namespace, pod, p.Status.Reason)
			}
			for _, c := range p.Status.Conditions {
				if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
					return nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for nested entry pod %s/%s: %w", namespace, pod, ctx.Err())
		case <-time.After(nestedReadyPoll):
		}
	}
}

// applyNestedManifests applies a module's manifest bundle into the nested
// cluster (B3): the vulnerable Deployments/RBAC/planted misconfig the student
// attacks. Uses a dynamic client + discovery RESTMapper so a module can ship
// any resource kind (including CRDs, if it applies them in an earlier doc)
// without the backend needing typed clients for each. Idempotent on
// AlreadyExists so a provision retry doesn't fail on a half-applied bundle.
func applyNestedManifests(ctx context.Context, cfg *rest.Config, manifests string) error {
	if strings.TrimSpace(manifests) == "" {
		return nil
	}
	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("nested dynamic client: %w", err)
	}
	disco, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return fmt.Errorf("nested discovery client: %w", err)
	}
	groups, err := restmapper.GetAPIGroupResources(disco)
	if err != nil {
		return fmt.Errorf("nested API discovery: %w", err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groups)

	dec := yaml.NewYAMLOrJSONDecoder(strings.NewReader(manifests), 4096)
	for {
		raw := map[string]any{}
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("decode manifest: %w", err)
		}
		if len(raw) == 0 {
			continue // empty document (e.g. trailing "---")
		}
		obj := &unstructured.Unstructured{Object: raw}
		gvk := obj.GroupVersionKind()
		mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("map %s: %w", gvk, err)
		}
		var ri dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			ns := obj.GetNamespace()
			if ns == "" {
				ns = "default"
			}
			ri = dyn.Resource(mapping.Resource).Namespace(ns)
		} else {
			ri = dyn.Resource(mapping.Resource)
		}
		if _, err := ri.Create(ctx, obj, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("apply %s %s: %w", gvk.Kind, obj.GetName(), err)
		}
	}
}

// createNestedDriverSecret writes the selected research drivers into a Secret in
// the nested cluster so the workstation pod's optional Secret volume can mount
// them read-only at /opt/drivers (see driver-research.yaml). It runs BEFORE the
// module manifest bundle brings up the workstation, so it ensures the target
// namespace idempotently first - the bundle's own Namespace create is harmless
// (AlreadyExists is success), matching applyNestedManifests. The Secret name
// must match the volume's secretName in the manifest. Secret.Data takes raw
// bytes (the API base64-encodes on the wire), so no encoding here.
func createNestedDriverSecret(ctx context.Context, cfg *rest.Config, namespace string, data map[string][]byte) error {
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("nested clientset: %w", err)
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
	if _, err := cs.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("ensure namespace %s: %w", namespace, err)
	}
	sec := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "research-drivers", Namespace: namespace},
		Data:       data,
	}
	if _, err := cs.CoreV1().Secrets(namespace).Create(ctx, sec, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create research-drivers secret: %w", err)
	}
	return nil
}
