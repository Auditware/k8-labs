package labs

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// A nested pod's caps are bounded by the runner's own bounding set. The research
// workstation insmods the target driver, so the runner must carry SYS_MODULE for
// research labs - and must NOT for any other lab (needless privilege). This
// guards the fix for the prod failure where the workstation container could not
// start ("unable to apply caps: operation not permitted").
func TestRunnerCapabilitiesSysModuleForResearchOnly(t *testing.T) {
	has := func(caps []corev1.Capability, want corev1.Capability) bool {
		for _, c := range caps {
			if c == want {
				return true
			}
		}
		return false
	}
	if !has(runnerCapabilities(Module{Kernel: KernelResearchDev}), "SYS_MODULE") {
		t.Error("research runner must include SYS_MODULE so the nested workstation can insmod")
	}
	for _, m := range []Module{{Slug: "normal"}, {Slug: "ebpf", EBPF: true}} {
		if has(runnerCapabilities(m), "SYS_MODULE") {
			t.Errorf("%s runner must NOT include SYS_MODULE (needless privilege)", m.Slug)
		}
	}
}

// The whole OOM guarantee rests on this one property: the scheduler admits
// against requests, so if the request is below what a session really uses, it
// will pack more sessions than the node can hold. Equal request and limit makes
// overcommit arithmetically impossible. This is the regression guard for
// somebody "optimising" the request back down to raise density.
func TestRunnerMemoryRequestEqualsLimit(t *testing.T) {
	for _, module := range []Module{
		{Slug: "normal"},
		{Slug: "ebpf", EBPF: true},
	} {
		t.Run(module.Slug, func(t *testing.T) {
			res := runnerResources(module)
			req := res.Requests[corev1.ResourceMemory]
			lim := res.Limits[corev1.ResourceMemory]
			if req.Cmp(lim) != 0 {
				t.Fatalf("memory request %s != limit %s: the scheduler could overcommit the node", req.String(), lim.String())
			}
		})
	}
}

// SessionMemory is the single source the pod spec, the admission budget and the
// usage meter all read. If runnerResources ever stops agreeing with it, the
// backend would admit sessions against one number while the scheduler places
// them against another - the failure would be silent until the node OOMed.
func TestRunnerMemoryMatchesSessionMemory(t *testing.T) {
	for _, module := range []Module{
		{Slug: "normal"},
		{Slug: "ebpf", EBPF: true},
	} {
		t.Run(module.Slug, func(t *testing.T) {
			want := SessionMemory(module)
			got := runnerResources(module).Requests[corev1.ResourceMemory]
			if got.Cmp(want) != 0 {
				t.Fatalf("pod requests %s but SessionMemory says %s", got.String(), want.String())
			}
		})
	}
}

// CPU must stay burstable. Pinning it the way memory is pinned would cap the
// AX41 at 12/2 = 6 sessions and buy nothing: CPU contention slows sessions, it
// does not kill them.
func TestRunnerCPUStaysBurstable(t *testing.T) {
	for _, module := range []Module{
		{Slug: "normal"},
		{Slug: "ebpf", EBPF: true},
	} {
		t.Run(module.Slug, func(t *testing.T) {
			res := runnerResources(module)
			req := res.Requests[corev1.ResourceCPU]
			lim := res.Limits[corev1.ResourceCPU]
			if req.Cmp(lim) >= 0 {
				t.Fatalf("cpu request %s >= limit %s: burst headroom is gone and capacity drops to 6 sessions", req.String(), lim.String())
			}
		})
	}
}

// An eBPF session really does cost more (measured 2.08 GiB vs 1.08 GiB on the
// AX41), and both the capacity budget and the usage price depend on that
// ordering holding.
func TestEBPFSessionReservesMoreThanNormal(t *testing.T) {
	normal := SessionMemory(Module{Slug: "normal"})
	ebpf := SessionMemory(Module{Slug: "ebpf", EBPF: true})
	if ebpf.Cmp(normal) <= 0 {
		t.Fatalf("ebpf reserves %s, normal reserves %s: eBPF must cost more", ebpf.String(), normal.String())
	}
}

// Headroom over measured usage, without being so generous that capacity
// collapses. Measured: 1.08 GiB normal, 2.08 GiB eBPF. Guarding both directions
// catches a future edit that either starves a session or halves the node.
func TestSessionMemoryKeepsHeadroomOverMeasuredUsage(t *testing.T) {
	cases := []struct {
		name          string
		module        Module
		measuredMiB   int64
		maxReserveMiB int64
	}{
		{"normal", Module{Slug: "normal"}, 1080, 2048},
		{"ebpf", Module{Slug: "ebpf", EBPF: true}, 2080, 3072},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := SessionMemory(tc.module)
			gotMiB := got.Value() / (1 << 20)
			if gotMiB <= tc.measuredMiB {
				t.Fatalf("reserves %d MiB but sessions measure %d MiB: no headroom, sessions will be OOM-killed", gotMiB, tc.measuredMiB)
			}
			if gotMiB > tc.maxReserveMiB {
				t.Fatalf("reserves %d MiB, more than the %d MiB ceiling: node capacity drops for no measured reason", gotMiB, tc.maxReserveMiB)
			}
		})
	}
}
