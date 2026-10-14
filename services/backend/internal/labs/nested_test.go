package labs

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func nestedTestModule() Module {
	return Module{
		Slug:   "ns-escape",
		Nested: &NestedSpec{Manifests: "apiVersion: v1\nkind: Namespace\nmetadata: {name: victim}\n", EntryPod: "foothold", EntryPort: 7681},
	}
}

// buildRunnerPod's security context is the local-vs-prod seam that A2/A3
// pinned down: privileged under runc (local), explicit capabilities under Kata
// (prod). Getting this wrong is exactly the failure mode A3 hit (privileged +
// Kata = host block-device hotplug crash), so assert both directions.
func TestBuildRunnerPod_RuncPrivileged(t *testing.T) {
	m := &Manager{cfg: Config{RunnerNamespace: "labs-runners", RunnerImage: "reg/session-runner:dev"}} // RuntimeClass empty = runc
	pod := m.buildRunnerPod("abc", "owner1", nestedTestModule())

	sc := pod.Spec.Containers[0].SecurityContext
	if sc.Privileged == nil || !*sc.Privileged {
		t.Errorf("runc runner: Privileged = %v, want true", sc.Privileged)
	}
	if sc.Capabilities != nil && len(sc.Capabilities.Add) > 0 {
		t.Errorf("runc runner: unexpected added caps %v (privileged already covers it)", sc.Capabilities.Add)
	}
	if pod.Spec.RuntimeClassName != nil {
		t.Errorf("runc runner: RuntimeClassName = %v, want nil", *pod.Spec.RuntimeClassName)
	}
}

func TestBuildRunnerPod_KataCapabilities(t *testing.T) {
	m := &Manager{cfg: Config{RunnerNamespace: "labs-runners", RunnerImage: "reg/session-runner:dev", RuntimeClass: "kata-fc"}}
	pod := m.buildRunnerPod("abc", "owner1", nestedTestModule())

	sc := pod.Spec.Containers[0].SecurityContext
	if sc.Privileged != nil && *sc.Privileged {
		t.Error("kata runner: Privileged must NOT be set - it triggers host block-device hotplug (A3)")
	}
	if sc.Capabilities == nil {
		t.Fatal("kata runner: expected added capabilities, got none")
	}
	// SYS_ADMIN is the load-bearing one (cgroup ns + mounts inside the guest);
	// its absence would reproduce the "evacuate root cgroup" failure from A3.
	if !hasCap(sc.Capabilities.Add, "SYS_ADMIN") {
		t.Errorf("kata runner: caps %v missing SYS_ADMIN", sc.Capabilities.Add)
	}
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "kata-fc" {
		t.Errorf("kata runner: RuntimeClassName = %v, want kata-fc", pod.Spec.RuntimeClassName)
	}
}

// An eBPF-opted module selects the dedicated kata-fc-ebpf RuntimeClass (an
// eBPF-capable guest kernel) instead of the default kata-fc, without changing
// the Kata security posture - that is the whole point of the per-session opt-in.
func TestBuildRunnerPod_EBPFRuntimeClass(t *testing.T) {
	m := &Manager{cfg: Config{RunnerNamespace: "labs-runners", RunnerImage: "reg/session-runner:dev", RuntimeClass: "kata-fc", RuntimeClassEBPF: "kata-fc-ebpf"}}

	mod := nestedTestModule()
	mod.EBPF = true
	pod := m.buildRunnerPod("abc", "owner1", mod)
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "kata-fc-ebpf" {
		t.Errorf("eBPF module: RuntimeClassName = %v, want kata-fc-ebpf", pod.Spec.RuntimeClassName)
	}
	// Still Kata (caps, not privileged) - the eBPF class only swaps the kernel.
	if sc := pod.Spec.Containers[0].SecurityContext; sc.Privileged != nil && *sc.Privileged {
		t.Error("eBPF kata runner: Privileged must NOT be set")
	}

	// A non-eBPF module on the same config keeps the default class.
	pod = m.buildRunnerPod("abc", "owner1", nestedTestModule())
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "kata-fc" {
		t.Errorf("non-eBPF module: RuntimeClassName = %v, want kata-fc", pod.Spec.RuntimeClassName)
	}
}

// eBPF sessions are heavier, so the runner reserves more memory (the scheduler's
// packing lever = the concurrency governor) and carries SESSION_EBPF=1 to gate
// the Tetragon airgap import in entrypoint.sh. Normal labs must carry neither.
func TestBuildRunnerPod_EBPFSizingAndImportGate(t *testing.T) {
	m := &Manager{cfg: Config{RunnerNamespace: "labs-runners", RunnerImage: "reg/session-runner:dev", RuntimeClass: "kata-fc", RuntimeClassEBPF: "kata-fc-ebpf"}}

	mod := nestedTestModule()
	mod.EBPF = true
	ebpf := m.buildRunnerPod("abc", "owner1", mod)
	normal := m.buildRunnerPod("abc", "owner1", nestedTestModule())

	ec := ebpf.Spec.Containers[0]
	nc := normal.Spec.Containers[0]

	// Heavier memory request than a normal lab (the concurrency lever).
	if ec.Resources.Requests.Memory().Cmp(*nc.Resources.Requests.Memory()) <= 0 {
		t.Errorf("eBPF mem request %v must exceed normal %v", ec.Resources.Requests.Memory(), nc.Resources.Requests.Memory())
	}
	if !hasEnv(ec.Env, "SESSION_EBPF", "1") {
		t.Error("eBPF runner missing SESSION_EBPF=1 (Tetragon import gate)")
	}
	if hasEnvName(nc.Env, "SESSION_EBPF") {
		t.Error("normal runner must NOT carry SESSION_EBPF (would import Tetragon and slow every boot)")
	}
}

// Locally (no Kata, no eBPF kernel - both classes empty) an eBPF module still
// runs: runc's host kernel already has BTF, so it falls back to the runc path.
// This is what makes Slices A-C testable with no prod kernel work.
func TestBuildRunnerPod_EBPFLocalFallsBackToRunc(t *testing.T) {
	m := &Manager{cfg: Config{RunnerNamespace: "labs-runners", RunnerImage: "reg/session-runner:dev"}} // both runtime classes empty

	mod := nestedTestModule()
	mod.EBPF = true
	pod := m.buildRunnerPod("abc", "owner1", mod)
	if pod.Spec.RuntimeClassName != nil {
		t.Errorf("eBPF module locally: RuntimeClassName = %v, want nil (runc)", *pod.Spec.RuntimeClassName)
	}
	if sc := pod.Spec.Containers[0].SecurityContext; sc.Privileged == nil || !*sc.Privileged {
		t.Errorf("eBPF module locally: Privileged = %v, want true (runc)", sc.Privileged)
	}
}

func TestBuildRunnerPod_Common(t *testing.T) {
	m := &Manager{cfg: Config{RunnerNamespace: "labs-runners", RunnerImage: "reg/session-runner:dev", NodeSelector: "k8labs.io/role=labs"}}
	pod := m.buildRunnerPod("abc", "owner1", nestedTestModule())

	if pod.Namespace != "labs-runners" {
		t.Errorf("namespace = %q, want labs-runners", pod.Namespace)
	}
	if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
		t.Error("runner must NOT automount a SA token (nothing for a nested escape to steal)")
	}
	if pod.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("restart policy = %v, want Never", pod.Spec.RestartPolicy)
	}
	c := pod.Spec.Containers[0]
	if c.Image != "reg/session-runner:dev" {
		t.Errorf("image = %q, want reg/session-runner:dev", c.Image)
	}
	if c.ImagePullPolicy != corev1.PullAlways {
		t.Errorf("pull policy = %v, want Always", c.ImagePullPolicy)
	}
	// SESSION_API_HOST must come from the downward API (pod IP) so the runner
	// rewrites its published kubeconfig to a reachable address.
	var found bool
	for _, e := range c.Env {
		if e.Name == "SESSION_API_HOST" {
			found = true
			if e.ValueFrom == nil || e.ValueFrom.FieldRef == nil || e.ValueFrom.FieldRef.FieldPath != "status.podIP" {
				t.Errorf("SESSION_API_HOST must be a downward-API ref to status.podIP, got %+v", e)
			}
		}
	}
	if !found {
		t.Error("missing SESSION_API_HOST env var")
	}
	if !labsNodeTolerated(pod.Spec.Tolerations) {
		t.Error("runner pod does not tolerate the labs-node taint")
	}
	if pod.Spec.NodeSelector["k8labs.io/role"] != "labs" {
		t.Errorf("nodeSelector = %v, want k8labs.io/role=labs", pod.Spec.NodeSelector)
	}
}

func TestRunnerLabels(t *testing.T) {
	l := runnerLabels("sess1", "owner1", Module{Slug: "ns-escape"})
	// app=session-runner is what Reconcile (B5) selects on to re-adopt orphaned
	// runners - distinct from legacy app=lab pods.
	if l["app"] != "session-runner" {
		t.Errorf("app label = %q, want session-runner", l["app"])
	}
	for k, want := range map[string]string{"session": "sess1", "owner": "owner1", "module": "ns-escape"} {
		if l[k] != want {
			t.Errorf("label %q = %q, want %q", k, l[k], want)
		}
	}
}

func hasEnv(env []corev1.EnvVar, name, value string) bool {
	for _, e := range env {
		if e.Name == name {
			return e.Value == value
		}
	}
	return false
}

func hasEnvName(env []corev1.EnvVar, name string) bool {
	for _, e := range env {
		if e.Name == name {
			return true
		}
	}
	return false
}

func hasCap(caps []corev1.Capability, want corev1.Capability) bool {
	for _, c := range caps {
		if c == want {
			return true
		}
	}
	return false
}

func labsNodeTolerated(tols []corev1.Toleration) bool {
	for _, t := range tols {
		if t.Key == "k8labs.io/dedicated" && t.Value == "labs" && t.Effect == corev1.TaintEffectNoSchedule {
			return true
		}
	}
	return false
}

// Egress is opt-in, and the DEFAULT is the security property worth pinning: the
// runner-allow-external-egress NetworkPolicy selects on k8labs.io/egress, so a
// runner without that label has no route off the box. Nested traffic SNATs
// through this pod, which means a regression here silently turns every lab into
// an egress node running from the labs node's own public IP.
func TestRunnerHasNoEgressLabelByDefault(t *testing.T) {
	for _, m := range ListModules() {
		l := runnerLabels("s1", "o1", m)
		if v, present := l["k8labs.io/egress"]; present {
			t.Errorf("module %s grants egress (label=%q) without declaring NeedsEgress", m.Slug, v)
		}
	}
}

// ...and when a module does declare it, the label must actually appear, or the
// opt-in silently does nothing and the lab breaks in a confusing way.
func TestRunnerGetsEgressLabelWhenModuleDeclaresIt(t *testing.T) {
	l := runnerLabels("s1", "o1", Module{Slug: "egress-lab", NeedsEgress: true})
	if l["k8labs.io/egress"] != "allowed" {
		t.Fatalf("k8labs.io/egress = %q, want \"allowed\"", l["k8labs.io/egress"])
	}
	if l["app"] != "session-runner" {
		t.Fatalf("app label lost: %q", l["app"])
	}
}
