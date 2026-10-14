package labs

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"
)

// Config controls how session pods are built - deliberately env-driven, same
// split as internal/db: differs between local/k3d and real Hetzner
// deployments, same code path either way.
type Config struct {
	// RunnerNamespace holds the per-session session-runner pods. It sits at
	// PodSecurity "privileged" because the runner is privileged (runc, local) or
	// high-capability (Kata, prod). The lab-content pods live *inside* the nested
	// cluster, governed by its own PSA, so this doesn't weaken anything.
	RunnerNamespace string
	// RunnerImage is the full ref of the session-runner image (the container
	// that boots a throwaway k3s - services/session-runner). e.g.
	// ghcr.io/auditware/session-runner:latest in prod, localhost registry locally.
	RunnerImage string
	// RuntimeClass is empty locally (no Kata installed on k3d) and
	// "kata-fc" in real environments once that
	// RuntimeClass exists. Empty means: omit RuntimeClassName entirely,
	// pod runs under the node's default runtime.
	RuntimeClass string
	// RuntimeClassEBPF is the RuntimeClass for eBPF-opted modules (Module.EBPF):
	// a microVM whose guest kernel exposes BTF/kprobes/tracefs so the student can
	// run eBPF tooling inside their own lab. "kata-fc-ebpf" in prod (a dedicated
	// class pointing at the eBPF kernel, keeping the default kata-fc lean), empty
	// locally - where runc's host kernel already has BTF, so an eBPF lab needs no
	// custom kernel. Empty = fall back to RuntimeClass for eBPF modules too (the
	// local runc path). See docs/architecture/ebpf-per-session.md.
	RuntimeClassEBPF string
	// RuntimeClassResearchDev / RuntimeClassResearchHard are the RuntimeClasses
	// for the kernel-driver research labs (Module.Kernel research-dev / -hard): a
	// microVM whose guest kernel is KASAN+KCOV+BTF+kprobes instrumented and IS the
	// exploitation target. Dev is unhardened (technique bring-up), hard carries
	// production mitigations. "kata-fc-research-dev" / "-hard" in prod, empty
	// locally (runc's host kernel - the plumbing is testable without the custom
	// kernel, exactly like eBPF). Empty = fall back to RuntimeClass.
	// See docs/architecture/kasan-research-lab.md.
	RuntimeClassResearchDev  string
	RuntimeClassResearchHard string
	// NodeSelector pins lab pods to a dedicated node - a single "key=value"
	// pair (empty means omit NodeSelector entirely, same pattern as
	// RuntimeClass). Locally this is a k3d agent node labeled by hand (see
	// docs/architecture/environments.md); in real environments it's the
	// Kata-fc node group. Mirrors prod's node-level separation between the
	// control plane and labs instead of leaving it purely namespace-deep.
	NodeSelector string
	// MaxSessionsPerUser caps concurrent sessions per student. 0 disables the
	// check; 1 is the default and is almost always right - a student drives one
	// terminal at a time, and a second session is usually an abandoned first one
	// still holding its full memory reservation.
	//
	// It bounds *concurrent* use only. Stopping and restarting to collect fresh
	// TTL windows is unaffected by design - that is the daily quota's job, since
	// the two limits answer different questions.
	MaxSessionsPerUser int
	// MemoryBudget is how much of the labs node the backend will hand out before
	// refusing new sessions, summed over SessionMemory of everything live.
	//
	// This is a UX gate, not the safety mechanism. The real guarantee is the
	// scheduler's, because runner pods request exactly what they limit (see
	// runnerResources) and it therefore cannot overcommit the node. But a
	// scheduler refusal looks like a pod stuck Pending behind a spinner forever,
	// which is the worst way to tell someone the platform is busy. So the budget
	// is set slightly BELOW what the scheduler would allow: the backend gets to
	// answer first, with a sentence and a Retry-After, and the scheduler stays as
	// the backstop that cannot be talked around.
	//
	// Zero disables the check. Size it per environment - the prod AX41 has ~60.8
	// GiB usable, while a local Docker VM may have under 3 GiB free, so the
	// default here suits prod and local overrides it downward via the chart.
	MemoryBudget resource.Quantity
	// DailyQuotaGiBMinutes caps one user's consumption over a rolling 24 hours.
	// 0 disables it.
	//
	// This closes the gap MaxSessionsPerUser leaves open. A concurrency cap only
	// bounds how many labs run at once; it does nothing about stopping and
	// restarting all day to collect fresh TTL windows, which costs the platform
	// exactly as much. The two limits answer different questions and neither
	// substitutes for the other.
	//
	// Priced in GiB-minutes so a heavier lab draws down faster: an eBPF session
	// spends the allowance ~1.67x as quickly as a normal one, because it really
	// does hold ~1.67x the memory for each minute it runs.
	DailyQuotaGiBMinutes float64
}

// nodeSelectorMap parses Config.NodeSelector's "key=value" form into the
// map client-go's PodSpec.NodeSelector wants. Returns nil (omit the field)
// if unset or malformed - a bad value should degrade to "no selector",
// not crash session provisioning.
func (c Config) nodeSelectorMap() map[string]string {
	key, value, ok := strings.Cut(c.NodeSelector, "=")
	if !ok || key == "" {
		return nil
	}
	return map[string]string{key: value}
}

// runtimeClassFor picks the RuntimeClass for a session's runner pod. eBPF
// modules need a guest kernel with BTF/kprobes/tracefs, exposed in prod as a
// dedicated RuntimeClass (kata-fc-ebpf) kept separate from the default kata-fc so
// the stock kernel keeps its lean, fast-booting surface. Locally both are empty
// (runc, whose host kernel already has BTF), so an eBPF lab runs with no custom
// kernel - which is why the backend/image/content slices are testable without any
// prod kernel work. See docs/architecture/ebpf-per-session.md.
func (c Config) runtimeClassFor(module Module) string {
	switch module.kernel() {
	case KernelResearchDev:
		if c.RuntimeClassResearchDev != "" {
			return c.RuntimeClassResearchDev
		}
	case KernelResearchHard:
		if c.RuntimeClassResearchHard != "" {
			return c.RuntimeClassResearchHard
		}
	case KernelEBPF:
		if c.RuntimeClassEBPF != "" {
			return c.RuntimeClassEBPF
		}
	}
	return c.RuntimeClass
}

// defaultMemoryBudget targets the prod labs node (AX41, ~60.8 GiB usable with
// the platform's own pods already running). 56Gi leaves headroom for the node's
// system pods and keeps the backend refusing just before the scheduler would -
// roughly 37 normal sessions or 22 eBPF ones. Local dev overrides it downward.
const defaultMemoryBudget = "56Gi"

func ConfigFromEnv() Config {
	return Config{
		RunnerNamespace:          getenvDefault("LABS_RUNNER_NAMESPACE", "labs-runners"),
		RunnerImage:              getenvDefault("LABS_RUNNER_IMAGE", "ghcr.io/ORG/session-runner:latest"),
		RuntimeClass:             os.Getenv("LABS_RUNTIME_CLASS"),
		RuntimeClassEBPF:         os.Getenv("LABS_RUNTIME_CLASS_EBPF"),
		RuntimeClassResearchDev:  os.Getenv("LABS_RUNTIME_CLASS_RESEARCH_DEV"),
		RuntimeClassResearchHard: os.Getenv("LABS_RUNTIME_CLASS_RESEARCH_HARD"),
		NodeSelector:             os.Getenv("LABS_NODE_SELECTOR"),
		MaxSessionsPerUser:       getenvInt("LABS_MAX_SESSIONS_PER_USER", 1),
		MemoryBudget:             getenvQuantity("LABS_MEMORY_BUDGET", defaultMemoryBudget),
		// 0 = off. Deliberately not enabled by default: the right number depends
		// on how people actually use the platform, and there is no data yet.
		// Metering runs regardless, so the threshold can be chosen from real
		// GiB-minute distributions rather than guessed now and then defended.
		//
		// For scale when you do set it: a full 30-minute normal lab is ~45
		// GiB-minutes, a full 45-minute eBPF lab ~112. So 500 is roughly ten
		// normal labs a day, which is generous for a student and still bounds the
		// worst case.
		DailyQuotaGiBMinutes: getenvFloat("LABS_DAILY_QUOTA_GIB_MINUTES", 0),
	}
}

// getenvFloat mirrors getenvInt's fallback reasoning.
func getenvFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		slog.Warn("labs config: bad float, using default", "key", key, "value", v, "default", fallback)
		return fallback
	}
	return f
}

func getenvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getenvInt falls back on anything unparseable rather than failing startup. A
// typo in a limit should leave the safe default in force, not take the backend
// down or - worse - silently read as 0 and disable the limit entirely.
func getenvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		slog.Warn("labs config: bad integer, using default", "key", key, "value", v, "default", fallback)
		return fallback
	}
	return n
}

// getenvQuantity parses a Kubernetes quantity ("56Gi"). Same fallback reasoning
// as getenvInt: an unparseable budget must not become an unlimited one.
func getenvQuantity(key, fallback string) resource.Quantity {
	v := os.Getenv(key)
	if v == "" {
		v = fallback
	}
	q, err := resource.ParseQuantity(v)
	if err != nil {
		slog.Warn("labs config: bad quantity, using default", "key", key, "value", v, "default", fallback)
		return resource.MustParse(fallback)
	}
	return q
}
