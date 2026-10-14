package labs

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// runnerName is the outer-cluster pod name for a session's session-runner
// (the pod that IS a throwaway nested k3s cluster). One per nested-shape
// session; deleting it takes the whole nested cluster with it (B4).
func runnerName(id string) string { return "runner-" + id }

// runnerLabels mirror sessionLabels but tag the runner pod so Reconcile (B5)
// can re-adopt orphaned runners after a restart, keyed on app=session-runner
// rather than app=lab (legacy pods).
func runnerLabels(id, ownerID string, module Module) map[string]string {
	l := map[string]string{
		"app":     "session-runner",
		"session": id,
		"module":  module.Slug,
		"owner":   ownerID,
	}
	// Egress is opt-in per module. The label is what the
	// runner-allow-external-egress NetworkPolicy selects on, so its ABSENCE is
	// the control: a runner without it matches no egress rule past DNS and stays
	// default-denied. Set only for modules declaring NeedsEgress - see that
	// field's note on why handing out internet is the interesting decision here.
	if module.NeedsEgress {
		l["k8labs.io/egress"] = "allowed"
	}
	return l
}

// runnerCapabilities are the capabilities the session-runner needs under Kata
// (A3): it is NOT privileged there, because a privileged pod makes Kata try to
// hotplug host block devices (incl. the devmapper loop devices) into the
// microVM and fails. These are the caps the nested k3s + kubelet + containerd
// need inside the guest, proven on the AX41. Under runc (local) we use
// privileged instead - see buildRunnerPod.
//
// A nested pod's capabilities are bounded by the runner's own bounding set, so a
// cap a lab pod needs must ALSO be on the runner. Research labs are the only ones
// whose workstation loads a kernel module (insmod), so SYS_MODULE is added for
// them alone - granting it to every runner would be needless privilege.
func runnerCapabilities(module Module) []corev1.Capability {
	caps := []corev1.Capability{
		"SYS_ADMIN",    // mount cgroups/containerd, unshare cgroup ns (cgns-exec)
		"NET_ADMIN",    // nested CNI / kube-proxy iptables
		"NET_RAW",      // nested networking
		"MKNOD",        // create /dev/kmsg etc in the guest
		"SYS_RESOURCE", // rlimits for the nested kubelet
		"SYS_PTRACE",   // kubelet/containerd introspection
		"SYS_CHROOT",   // container runtime pivot_root inside the nested cluster
	}
	if module.IsResearch() {
		// So the nested workstation can insmod the target driver into the guest
		// (KASAN) kernel - the whole point of the research labs.
		caps = append(caps, "SYS_MODULE")
	}
	return caps
}

// SessionMemory is what one session of this module reserves on the labs node.
// It is deliberately the ONLY place that number lives, because three otherwise
// unrelated decisions have to agree on it or the system misbehaves quietly: the
// runner pod's memory request and limit (below), the capacity admission budget
// (capacity.go), and the usage meter that prices a session in GiB-minutes
// (usage.go). One source, so they cannot drift apart.
//
// The values are measured on the AX41 under Kata/Firecracker (2026-07-23), not
// estimated. A full session - microVM plus nested k3s plus the lab's own
// manifests - costs:
//
//	normal lab   1.08 GiB   (1049 MiB Firecracker RSS + lab content)
//	eBPF lab     2.08 GiB   (1929 MiB + Tetragon and its BPF maps)
//
// The reservations carry ~40% headroom over those. Worth knowing: what makes an
// eBPF session expensive is the guest KERNEL, not the workload - the interim
// -debug kernel is ~880 MiB heavier than the lean one. Replacing it (see
// docs/architecture/ebpf-per-session.md) would bring eBPF close to normal cost
// and roughly double eBPF concurrency, and this is the number to revisit then.
//
// Do not infer these from a local run: local is runc with no microVM, where the
// same sessions measure 427 MiB and 596 MiB. The VM overhead is the difference,
// and it is the majority of the cost.
func SessionMemory(module Module) resource.Quantity {
	switch module.kernel() {
	case KernelResearchDev, KernelResearchHard:
		// KASAN roughly doubles kernel RAM on top of the eBPF kernel's surface,
		// so the research microVM is the heaviest tier. Measure and tune on the
		// AX41 first-boot; grant-gating keeps concurrency low enough to fit.
		return resource.MustParse("3072Mi")
	case KernelEBPF:
		return resource.MustParse("2560Mi")
	}
	return resource.MustParse("1536Mi")
}

// runnerResources sizes the runner pod.
//
// Memory request EQUALS limit, on purpose. The scheduler admits pods against
// requests, so a request below real usage lets it pack more sessions than the
// node can hold - which is exactly what used to happen here: 512Mi requested
// against ~1.08 GiB actually used, a 2.1x undercount, with the Firecracker
// overhead invisible on top of that. Setting request = limit makes overcommit
// arithmetically impossible, and it inverts the failure mode that matters: a
// session that outgrows its reservation is OOM-killed ALONE, where before it
// could take the whole node - and with it everybody else's lab - down with it.
//
// CPU is deliberately NOT pinned that way and stays burstable. CPU is
// time-shared: contention makes two sessions briefly slower, it does not kill
// anything, and pinning request to limit would cap the node at 6 sessions
// (12 cores / 2) for no safety gain. Memory is the resource that has to be
// reserved because it cannot be reclaimed under contention. Measured idle draw
// is only ~48 mcore (normal) and ~60 mcore (eBPF), so these requests already
// carry generous burst room for a student actually working.
//
// Net effect on the AX41: memory is the binding constraint at ~40 normal or
// ~24 eBPF sessions, which is what the doc comment above this function always
// claimed but did not previously achieve.
func runnerResources(module Module) corev1.ResourceRequirements {
	mem := SessionMemory(module)
	req := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("250m"),
		corev1.ResourceMemory: mem,
	}
	lim := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("2"),
		corev1.ResourceMemory: mem.DeepCopy(),
	}
	if module.usesEBPFKernel() {
		// Heavier guest kernel and a Tetragon agent to schedule around, so more
		// CPU floor and ceiling. Memory already differs via SessionMemory.
		req[corev1.ResourceCPU] = resource.MustParse("500m")
		lim[corev1.ResourceCPU] = resource.MustParse("2500m")
	}
	return corev1.ResourceRequirements{Requests: req, Limits: lim}
}

// buildRunnerPod constructs the session-runner pod: the container that boots a
// disposable nested k3s (services/session-runner). The spec is exactly what
// A2 (runc/k3d) and A3 (Firecracker/AX41) proved, with the one difference
// between them keyed off the RuntimeClass seam:
//
//   - local (RuntimeClass == ""): plain runc, privileged: true.
//   - prod  (RuntimeClass == "kata-fc"): microVM, NOT privileged, explicit
//     capabilities instead (see runnerCapabilities).
//
// eBPF-opted modules (Module.EBPF) select a distinct RuntimeClass instead
// (kata-fc-ebpf in prod - an eBPF-capable guest kernel), resolved by
// Config.runtimeClassFor. That only changes the guest *kernel*; the runc-vs-Kata
// security posture below still keys off whether the resolved class is non-empty,
// so an eBPF lab runs runc-privileged locally (host kernel already has BTF) and
// Kata-caps in prod, same as any other module.
//
// Everything else (image, env, mounts, node pinning) is identical. The image's
// own entrypoint handles the guest fixups (cgroup ns, /proc/sys remount,
// /dev/kmsg, native snapshotter, distinct nested CIDRs) so nothing here needs
// to - see services/session-runner/entrypoint.sh.
func (m *Manager) buildRunnerPod(id, ownerID string, module Module) *corev1.Pod {
	name := runnerName(id)
	runtimeClass := m.cfg.runtimeClassFor(module)
	kata := runtimeClass != ""

	sc := &corev1.SecurityContext{}
	if kata {
		sc.Capabilities = &corev1.Capabilities{Add: runnerCapabilities(module)}
	} else {
		sc.Privileged = boolPtr(true)
	}

	env := []corev1.EnvVar{
		{
			// Rewrites the published kubeconfig's server address from loopback to
			// this pod's IP so the backend can dial the nested API (entrypoint.sh).
			Name: "SESSION_API_HOST",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
			},
		},
	}
	if module.usesEBPFKernel() {
		// Gate the (heavier) Tetragon airgap import to eBPF sessions only, so
		// normal labs keep their fast boot even though the tar is baked into the
		// shared runner image. entrypoint.sh imports tetragon.tar only when set.
		env = append(env, corev1.EnvVar{Name: "SESSION_EBPF", Value: "1"})
	}
	if module.IsResearch() {
		// Same split for the research workstation image (research.tar): its
		// ~200 MB exploitation toolchain is baked into the shared runner image
		// but imported into the nested cluster only for the research labs, whose
		// workstation pod pulls k8labs-research with imagePullPolicy: Never.
		env = append(env, corev1.EnvVar{Name: "SESSION_RESEARCH", Value: "1"})
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: m.cfg.RunnerNamespace,
			Labels:    runnerLabels(id, ownerID, module),
		},
		Spec: corev1.PodSpec{
			// The runner needs no outer-cluster identity: the backend reads its
			// nested kubeconfig by exec, and the nested cluster has its own
			// service accounts. No token = nothing for a nested-cluster escape
			// to steal and replay against the OUTER API.
			AutomountServiceAccountToken: boolPtr(false),
			RestartPolicy:                corev1.RestartPolicyNever,
			NodeSelector:                 m.cfg.nodeSelectorMap(),
			Tolerations:                  labsTolerations(),
			Containers: []corev1.Container{
				{
					Name:  "runner",
					Image: m.cfg.RunnerImage,
					// Moving tags (:dev locally, :latest in prod) - always pull
					// so a rebuilt runner image is never served stale (A2).
					ImagePullPolicy: corev1.PullAlways,
					Env:             env,
					Ports: []corev1.ContainerPort{
						{Name: "kube-api", ContainerPort: 6443},
					},
					SecurityContext: sc,
					Resources:       runnerResources(module),
					// Ready once k3s is listening on the API port. Create then
					// separately waits for the nested node itself to go Ready
					// (waitForNestedReady) before declaring the session up.
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(6443)},
						},
						InitialDelaySeconds: 3,
						PeriodSeconds:       3,
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "rancher", MountPath: "/var/lib/rancher/k3s"},
						{Name: "kubelet", MountPath: "/var/lib/kubelet"},
						{Name: "run", MountPath: "/run"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{Name: "rancher", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "kubelet", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
				{Name: "run", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}},
			},
		},
	}
	if kata {
		pod.Spec.RuntimeClassName = &runtimeClass
	}
	return pod
}
