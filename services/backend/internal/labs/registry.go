// Package labs owns the module catalog and the lifecycle of per-session lab
// clusters (create/delete via client-go, plus the terminal bridge into each
// session's nested cluster).
package labs

import (
	"sort"
	"time"
)

// Module is hardcoded for now - there's no Postgres catalog table yet.
// Once that lands, ListModules should read from it instead.
//
// Every module is nested-shape: the session is a throwaway k3s cluster in its
// own microVM (the session-runner), and the lab is a bundle of manifests applied
// *into* that cluster (Nested.Manifests), so "winning" compromises only the
// disposable nested cluster. The terminal attaches to a pod inside it
// (Nested.EntryPod). See docs/architecture/nested-lab-rewrite.md.
//
// The older single-pod shape (a lab pod + a lateral-movement target in the outer
// labs namespace, gated by PodSecurity level) was removed - see
// docs/architecture/lab-session-rewrite.md, F1.
type Module struct {
	Slug        string `json:"slug"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Hint        string `json:"hint"` // a nudge, not the answer - see services/labs/<slug>/README.md for the full walkthrough
	// Nested carries the disposable-cluster spec. Required: Create rejects a
	// module without one (see session.go).
	Nested *NestedSpec `json:"nested,omitempty"`
	// EBPF opts this module into an eBPF-capable guest kernel (BTF/kprobes/
	// tracefs) so the student can run eBPF tooling - Tetragon, bpftrace - inside
	// their own lab microVM. Per-session opt-in by design: only eBPF labs pay the
	// heavier kernel, every other lab keeps the lean, fast-booting stock one. The
	// backend expresses this by selecting a dedicated RuntimeClass for the runner
	// pod (see Config.runtimeClassFor / buildRunnerPod); the eBPF caps and mounts
	// themselves live on the *nested* lab pod in Nested.Manifests, governed by the
	// nested cluster's PodSecurity. See docs/architecture/ebpf-per-session.md.
	EBPF bool `json:"ebpf,omitempty"`
	// Kernel selects the guest kernel (and thus RuntimeClass) this module's
	// session runs on, generalizing EBPF. Empty is the default lean kernel;
	// "ebpf" is the eBPF-capable kernel (equivalent to EBPF: true, kept for the
	// existing modules); "research-dev" / "research-hard" are the KASAN+KCOV+BTF
	// instrumented kernels for the driver-research labs (the guest kernel is the
	// exploitation target). Selected via Config.runtimeClassFor. The research
	// kernels are KASAN+eBPF, so they also take the eBPF caps/mounts path (see
	// usesEBPFKernel). See docs/architecture/kasan-research-lab.md.
	Kernel KernelKind `json:"kernel,omitempty"`
	// ExpectedVermagic is the vermagic string a .ko must carry to insmod into
	// this module's guest kernel. Surfaced to the client so the driver picker can
	// flag an incompatible module BEFORE the session starts, instead of letting
	// insmod fail opaquely mid-session.
	//
	// It is load-bearing rather than cosmetic: the research kernels build with
	// MODVERSIONS off (that is why insmod works at all), so vermagic is the ONLY
	// compatibility check standing - see docs/architecture/research-driver-
	// catalog.md, which calls it "the real compatibility gate".
	//
	// Empty for every non-research module, since nothing else mounts user
	// drivers. Filled from researchVermagic in init().
	ExpectedVermagic string `json:"expectedVermagic,omitempty"`
	// NeedsEgress opts this module's sessions into internet access. Default false,
	// and it should stay false for anything that does not specifically teach
	// outbound behaviour.
	//
	// Nested clusters are fully airgapped (the runner image bakes every image tar
	// they need), so no current lab requires this. What granting it does is hand
	// the student a general-purpose egress node: because nested traffic SNATs
	// through the runner pod, whatever they do leaves from the labs node's public
	// IP. Sessions are handed root deliberately, so treat this as the switch that
	// makes abuse worth attempting, and set it only with that in mind.
	//
	// Expressed as the k8labs.io/egress label on the runner pod, which is what
	// the runner-allow-external-egress NetworkPolicy selects on.
	NeedsEgress bool `json:"-"`
	// Timing, if set, is this module's own default session length - the middle
	// layer between DefaultTiming and an admin's lab_settings override (see
	// ModuleTiming). Only the fields that differ need setting; Clamp fills the
	// rest from DefaultTiming, so a module that only wants a shorter lab says
	// just MaxAge and inherits the shared idle timeout.
	//
	// Worth setting for most labs. Length should track how long the lab actually
	// takes, and it is also the main capacity lever: a session holds its whole
	// memory reservation (SessionMemory) for its entire life whether or not the
	// student is typing, so an hour granted to a twenty-minute lab is forty
	// minutes of a seat nobody else can use.
	Timing *Timing `json:"-"`
	// Order places the module in the catalog (the sidebar, the lab list), lowest
	// first. Explicit because `modules` below is a map and Go deliberately
	// randomises map iteration: without a sort key ListModules returned a
	// different order on every call, so the sidebar visibly reshuffled whenever
	// its query refetched (picking a lab was enough to trigger it). The value is
	// curriculum order, not difficulty metadata - it is not serialised, since the
	// array order already carries it to the client.
	Order int `json:"-"`
	// Track is the tactic bucket the sidebar groups this module under (see
	// trackOf). Unlike Order, it IS serialised: the frontend renders the catalog
	// as collapsible track sections (First Flight, then Recon, Privilege
	// Escalation, ...) rather than one flat list, so it needs the label per
	// module. Order still sorts labs WITHIN a track. Assigned centrally in
	// init() from trackOf so the buckets live in one readable place, not smeared
	// across every module literal.
	Track string `json:"track"`
	// HasDefender tells the frontend to show the Defender tab: true iff this
	// module ships an in-guest sensor whose stream the Defender pane tails
	// (Nested.Defender != nil). Computed centrally in init(), not authored per
	// module. It is deliberately NOT the EBPF flag: the two coincided until the
	// research labs, which have a Defender pane but select their kernel via
	// Kernel, not EBPF - gating the tab on EBPF would hide their sensor. Gating on
	// "does it actually have a defender" is the real question and stays correct as
	// the kernel taxonomy grows. See docs/architecture/kasan-research-lab.md.
	HasDefender bool `json:"defender,omitempty"`
	// Flag is the CTF flag the student captures by completing the lab - the exact
	// value the module's manifest plants (a k8labs{...} Secret, or a file dropped
	// on the nested node). It is the single server-side source of truth the flag
	// endpoint checks a submission against, and it is deliberately json:"-": the
	// answer must never ship to the browser, or the capstone proves nothing. It is
	// NOT authored into the course content for the same reason. A test
	// (TestModuleFlagMatchesManifest) asserts every module's Flag appears verbatim
	// in its Manifests so the two can never drift.
	Flag string `json:"-"`
}

// KernelKind selects a module's guest kernel, and with it the RuntimeClass its
// session runs on (see Config.runtimeClassFor). Empty is the default lean
// kernel; the rest opt into a heavier instrumented kernel that in prod is a
// dedicated RuntimeClass and locally is empty (runc's host kernel), so the
// backend/image/UI work is testable without any custom-kernel prod work.
type KernelKind string

const (
	KernelDefault      KernelKind = ""              // kata-fc
	KernelEBPF         KernelKind = "ebpf"          // kata-fc-ebpf
	KernelResearchDev  KernelKind = "research-dev"  // kata-fc-research-dev (KASAN+KCOV+BTF, unhardened)
	KernelResearchHard KernelKind = "research-hard" // kata-fc-research-hard (KASAN+KCOV+BTF, production mitigations)
)

// kernel resolves the module's effective kernel, bridging the legacy EBPF bool
// into the KernelKind selector: an explicit Kernel wins, else EBPF: true means
// KernelEBPF, else the default.
func (m Module) kernel() KernelKind {
	if m.Kernel != "" {
		return m.Kernel
	}
	if m.EBPF {
		return KernelEBPF
	}
	return KernelDefault
}

// usesEBPFKernel reports whether the module's kernel carries the eBPF surface
// (BTF/kprobes/tracefs + Tetragon). True for the eBPF kernel and for the
// research kernels (which are KASAN+eBPF), so both take the same caps/mounts,
// heavier resources, and Tetragon airgap import in runner.go / usage.go.
func (m Module) usesEBPFKernel() bool {
	switch m.kernel() {
	case KernelEBPF, KernelResearchDev, KernelResearchHard:
		return true
	}
	return false
}

// IsResearch reports whether the module is a kernel-driver research lab. These
// are discovery labs with no planted flag, exempt from the flag-capstone checks,
// and (being human-driven fuzzing sessions rather than guided hint windows) they
// take the longer AI-grant TTL - see internal/api/ai.go.
func (m Module) IsResearch() bool {
	k := m.kernel()
	return k == KernelResearchDev || k == KernelResearchHard
}

// NestedSpec describes a module that runs against a disposable nested k3s
// cluster (see Module's doc and docs/architecture/nested-lab-rewrite.md).
type NestedSpec struct {
	// Manifests is a multi-document YAML bundle applied into the nested
	// cluster once it's up (the lab's vulnerable Deployments, ServiceAccounts,
	// Roles, the planted misconfig, etc. - the target the student attacks).
	// Applied by the backend via the nested kubeconfig (see B3), NOT by the
	// outer cluster's ArgoCD - the nested cluster is created per session and
	// gone at teardown.
	Manifests string `json:"-"`
	// EntryPod / EntryNamespace / EntryPort name the pod *inside the nested
	// cluster* whose ttyd the terminal attaches to (the student's starting
	// foothold). The manifests above must create it. Used by the terminal
	// bridge (D1), not by provisioning itself.
	EntryNamespace string `json:"-"`
	EntryPod       string `json:"-"`
	EntryPort      int32  `json:"entryPort"`
	// AttackPath, if set, is the escalation route drawn on the student topology
	// for labs where it can't be derived. The escape-edge heuristic only finds a
	// cross-namespace path to a Secret (ns-escape); a local privilege
	// escalation (suid-privesc) has no such path, so the module names its own.
	// From/To are wiring keys the topology renderer resolves - a pod app/name, or
	// "ns:<name>" for a namespace - and Label rides the edge.
	AttackPath *AttackPath `json:"-"`
	// Defender, if set (Module.EBPF modules), names the in-guest eBPF sensor pod
	// whose event stream the Defender pane tails. The manifests must create it.
	// Structurally like EntryPod above, but for a log stream rather than an exec:
	// the backend follows the sensor's stdout via the nested kubeconfig and pumps
	// it to the browser (see defender.go, docs/architecture/ebpf-per-session.md).
	Defender *DefenderSpec `json:"-"`
}

// DefenderSpec locates the in-guest sensor's event stream for the Defender pane.
// PodLabel selects the pod (a DaemonSet's pod name is generated), Container is
// the one that writes JSON events to stdout (the export-stdout sidecar).
type DefenderSpec struct {
	Namespace string
	PodLabel  string // "key=value" selector, e.g. "app=tetragon"
	Container string
}

// AttackPath is a module's declared escalation route (see NestedSpec.AttackPath).
type AttackPath struct {
	From  string
	To    string
	Label string
}

var modules = map[string]Module{
	"ns-escape": {
		Order: 10,
		// Enumerate a service account, spot the ClusterRoleBinding, read the
		// Secret. 30 minutes is generous for that; the old 60 mostly bought
		// abandoned tabs.
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "Namespace Escape",
		Flag:        "k8labs{cross_ns_secret_read_via_overbroad_clusterrolebinding}",
		Description: "You have a shell in a dev-namespace pod whose service account was granted secret read a little too broadly. Read the flag Secret in the production namespace you were never meant to reach.",
		Hint:        "Your pod runs as the `analyst` service account with its token mounted. Enumerate what it can do (`kubectl auth can-i --list`), pay attention to how its secret-read permission was bound, and try it against a namespace other than your own.",
		Nested: &NestedSpec{
			Manifests:      nsEscapeManifests,
			EntryNamespace: "dev",
			EntryPod:       "foothold",
			EntryPort:      0, // exec-based terminal (D1), no in-pod ttyd port
		},
	},
	// The second lab, and the first proof that authoring is dynamic rather than
	// one-off: a different vulnerability class (local SUID escalation, CWE-78)
	// in the same nested shape, carrying its own manifest bundle and its own
	// embedded course. Recovers the lesson an earlier, since-removed SUID lab
	// taught, rebuilt as manifests-only. See modules/suid-privesc.yaml.
	"suid-privesc": {
		Order: 20,
		// Read the binary's behaviour, inject through the unsanitised argument,
		// read the flag. Comparable in length to ns-escape.
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "SUID Privilege Escalation",
		Flag:        "k8labs{suid_command_injection_to_root_via_backup_tool}",
		Description: "You have an unprivileged shell on a machine. A root-owned SUID binary, backup-tool, hands your argument to a shell without sanitizing it. Exploit the command injection for a root shell and read the flag only root can see.",
		Hint:        "Run `backup-tool` with no arguments to see what it does, then look at how it treats the filename you give it. It builds `cp <your arg> /backup/` and runs it through a shell - so what happens if your argument contains a `;`? It already runs as root, so an injected command - or a whole shell - runs as root too.",
		Nested: &NestedSpec{
			Manifests:      suidPrivescManifests,
			EntryNamespace: "lab",
			EntryPod:       "workstation",
			EntryPort:      0,
			// A local escalation has no cross-namespace route to derive, so the
			// lab names it. From and To are the SAME pod on purpose: the
			// workstation escalates to root in place, with no second pod or
			// namespace to travel to - the renderer draws that as a loop on the
			// pod. Pointing To at the namespace ("ns:lab") instead resolved to the
			// hive centre, which landed the arrow on whatever resource hex sat
			// nearest it (a configmap) and read as "SUID -> configmaps".
			AttackPath: &AttackPath{From: "workstation", To: "workstation", Label: "SUID → root"},
		},
	},
	// The first eBPF module (Module.EBPF): the student runs a SUID exec and
	// watches an in-guest Tetragon sensor catch the commit_creds transition -
	// the eBPF is the product, streamed to their own Defender pane. EBPF=true
	// selects the kata-fc-ebpf RuntimeClass (an eBPF-capable guest kernel) in
	// prod; locally runc's host kernel already has BTF. See
	// docs/architecture/ebpf-per-session.md and modules/ebpf-detect.yaml.
	"ebpf-detect": {
		Order: 30,
		// Longer than the other two because there is genuinely more to do: run
		// the escalation, then read the sensor's output and interpret it. Not
		// longer still, because an eBPF session reserves 2.5 GiB against a
		// normal lab's 1.5 GiB - every extra minute here costs ~1.7x what it
		// does elsewhere (see SessionMemory).
		Timing:      &Timing{MaxAge: 45 * time.Minute},
		DisplayName: "eBPF Detection",
		Flag:        "k8labs{ebpf_caught_the_commit_creds_transition}",
		Description: "You have an unprivileged shell, and an eBPF security sensor (Tetragon) running in your own lab. Escalate to root via the SUID backup-tool, then read the flag off the sensor's alert - it catches the kernel installing root credentials, the privilege jump that leaves no trace in the shell.",
		Hint:        "Read /app/README, then escalate with `backup-tool \"x; id\"` and switch to the Defender pane. Exec'ing a SUID-root binary makes the kernel call commit_creds() to install euid 0 - that is the event the sensor hooks, even though the shell shows nothing unusual. The flag is printed on that escalation alert; there is no flag file to read.",
		EBPF:        true,
		Nested: &NestedSpec{
			Manifests:      ebpfDetectManifests,
			EntryNamespace: "lab",
			EntryPod:       "workstation",
			EntryPort:      0, // exec-based terminal, no in-pod ttyd port
			// Same-pod escalation as suid-privesc above - see the note there
			// on why From and To are both the workstation pod.
			AttackPath: &AttackPath{From: "workstation", To: "workstation", Label: "SUID → euid 0 (detected)"},
			// The Defender pane tails the in-guest Tetragon sensor's export-stdout
			// sidecar (see modules/ebpf-detect.yaml).
			Defender: &DefenderSpec{Namespace: "tetragon", PodLabel: "app=tetragon", Container: "export-stdout"},
		},
	},
	// The RBAC-escalation track (Order 11-14): four distinct apiserver-permission
	// abuses, each attacked from a shell with the mounted service-account token,
	// each hitting the real authorizer - the same nested shape as ns-escape.
	//
	// Impersonation: a ClusterRole granting `impersonate` on users and groups is
	// bound to an otherwise-powerless service account. The student impersonates
	// the `system:masters` group, which the API server treats as superuser, and
	// reads a Secret their own identity cannot. See modules/sa-impersonate.yaml.
	"sa-impersonate": {
		Order: 40,
		// Enumerate the verb, name the group, read the secret. As short as
		// ns-escape.
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "Service Account Impersonation",
		Flag:        "k8labs{impersonate_a_group_to_become_system_masters}",
		Description: "You have a shell in a dev-namespace pod whose service account can impersonate other identities but can read nothing itself. Read the flag Secret in the production namespace by borrowing an identity you were never meant to hold.",
		Hint:        "Your service account (`sa-imper`) can't read secrets anywhere - check with `kubectl auth can-i --list`. But look at what verb it *does* have. `impersonate` lets you act as any user or group, and one group - `system:masters` - is hardwired into the API server as superuser. `kubectl` exposes this as `--as` and `--as-group`.",
		Nested: &NestedSpec{
			Manifests:      saImpersonateManifests,
			EntryNamespace: "dev",
			EntryPod:       "foothold",
			EntryPort:      0, // exec-based terminal (D1), no in-pod ttyd port
			// Impersonation is not the bound-SA cross-namespace shape the topology
			// heuristic derives, so the module names its own edge: the dev foothold
			// reaching the production Secret by way of a borrowed identity.
			AttackPath: &AttackPath{From: "foothold", To: "ns:production", Label: "impersonate system:masters"},
		},
	},
	// The recon track (Order 41-43): three labs that drill the reconnaissance an
	// attacker runs the moment they land on a pod - what you have, who you are,
	// what is around you - before any escalation. Deliberately kubectl-free (the
	// foothold removes it at start): a compromised workload has curl, not a
	// configured kubectl, so the student learns to talk to the API and the
	// network with the raw token. Each still ends in a captured k8labs{...} flag.
	//
	// R1 First Contact: pure local recon, no network. A Secret the pod was handed
	// leaks into its own environment (a secretKeyRef the app never needed). The
	// student inventories the pod - token, namespace, env, resolv.conf - and finds
	// the flag was in their pocket the whole time. See modules/recon-first-contact.yaml.
	"recon-first-contact": {
		Order:       41,
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "First Contact",
		Flag:        "k8labs{the_pod_handed_you_a_secret_in_its_own_env}",
		Description: "You just landed a shell in a pod. Before you touch the network, take inventory of what the pod already handed you - its identity, its environment, its mounts. One of them is a secret the app never should have carried. Find it.",
		Hint:        "You do not need the API or the network yet. Read what the pod gave you: `env` lists every environment variable, and this pod has a database password injected from a Secret via `secretKeyRef`. `/var/run/secrets/kubernetes.io/serviceaccount/namespace` tells you where you are, and `/etc/resolv.conf` confirms the cluster domain. The flag is sitting in your environment.",
		Nested: &NestedSpec{
			Manifests:      reconFirstContactManifests,
			EntryNamespace: "dev",
			EntryPod:       "foothold",
			EntryPort:      0,
			// No escalation and no travel: the secret is already inside the pod.
			// Drawn as a loop on the foothold, like the local-escalation labs.
			AttackPath: &AttackPath{From: "foothold", To: "foothold", Label: "read your own env"},
		},
	},
	// R2 Who Am I: the first curl-to-API lab. kubectl is gone, so the student
	// hits the apiserver with the mounted token. They ask who they are
	// (SelfSubjectReview), enumerate namespaces, and ask what they can do in each
	// (SelfSubjectRulesReview) - which surfaces a get-secrets grant in `billing`
	// that nothing else advertised. See modules/recon-who-am-i.yaml.
	"recon-who-am-i": {
		Order:       42,
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "Who Am I",
		Flag:        "k8labs{selfsubjectrulesreview_reveals_the_grant_you_hold}",
		Description: "Your pod has no kubectl - just like a real compromised workload. All you have is a service-account token and curl. Use them to ask the API who you are and what you are allowed to do, then follow the one permission you were quietly given to a secret you were never meant to read.",
		Hint:        "The token at `/var/run/secrets/kubernetes.io/serviceaccount/token` plus `curl --cacert .../ca.crt` is kubectl. POST a `SelfSubjectRulesReview` (in `authorization.k8s.io/v1`) for each namespace to list your own permissions - you can enumerate namespaces first with a GET on `/api/v1/namespaces`. One namespace grants you `get` on secrets. Read the flag Secret there with curl.",
		Nested: &NestedSpec{
			Manifests:      reconWhoAmIManifests,
			EntryNamespace: "dev",
			EntryPod:       "foothold",
			EntryPort:      0,
			// The discovered grant reaches a secret in the billing namespace - a
			// permissions-review path, not a bound-SA escape the heuristic derives.
			AttackPath: &AttackPath{From: "foothold", To: "ns:billing", Label: "rules review → read secret"},
		},
	},
	// R3 The Neighborhood: recon on the network plane. The foothold can list
	// nothing over the API (a 403), but DNS answers anyone. The pod's own mounted
	// config names an internal service in another namespace; the student resolves
	// it by name and curls it for the flag - proving `list` denial is not
	// isolation. See modules/recon-neighborhood.yaml.
	"recon-neighborhood": {
		Order:       43,
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "The Neighborhood",
		Flag:        "k8labs{dns_named_the_service_rbac_tried_to_hide}",
		Description: "The API server tells you nothing - your account cannot list services or pods. But DNS never checks your permissions, and the app you compromised left its own config behind. Map the neighborhood from what the pod already knows and reach an internal service you were never allowed to see.",
		Hint:        "Listing services over the API returns 403 - that is not isolation. Read the app's config mounted at `/etc/app/` - it names an upstream service by its cluster DNS name. Resolve it with `nslookup` or `getent hosts` (DNS answers regardless of RBAC), then `curl` that service. The flag is in the response body.",
		Nested: &NestedSpec{
			Manifests:      reconNeighborhoodManifests,
			EntryNamespace: "dev",
			EntryPod:       "foothold",
			EntryPort:      0,
			AttackPath:     &AttackPath{From: "foothold", To: "ns:observability", Label: "DNS → curl internal service"},
		},
	},
	// Pod exec: the foothold holds `pods/exec` in the ops namespace but reads
	// nothing itself. It execs into a CI pod running as a service account that
	// can read the production flag, inheriting that identity. See
	// modules/pod-exec.yaml.
	"pod-exec": {
		Order: 50,
		// Find the pod, exec in, read the secret with the borrowed token. As
		// short as ns-escape.
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "Pod Exec Lateral Movement",
		Flag:        "k8labs{pods_exec_inherits_the_targets_service_account}",
		Description: "You have a shell whose service account can exec into pods in the ops namespace but can read nothing itself. A CI pod there runs as an identity that can read the flag Secret in production. Exec into it and use its token to read the flag.",
		Hint:        "Check `kubectl auth can-i --list` - you hold `create` on `pods/exec` in `ops`, nothing more. Exec doesn't just run a command; it drops you inside the target pod, where that pod's service-account token is mounted. Find the pod (`kubectl get pods -n ops`), exec in, and use the token you find there.",
		Nested: &NestedSpec{
			Manifests:      podExecManifests,
			EntryNamespace: "dev",
			EntryPod:       "foothold",
			EntryPort:      0,
			// Lateral movement, not a bound-SA cross-namespace read, so the module
			// names its own edge: the dev foothold execing into the ops CI pod.
			AttackPath: &AttackPath{From: "foothold", To: "ci-runner", Label: "pods/exec → borrow token"},
		},
	},
	// Create pods: the foothold can create pods in ops but reads nothing. It
	// launches a pod that runs AS a privileged service account (builder) and
	// prints the production flag to its logs. See modules/create-pods.yaml.
	"create-pods": {
		Order: 60,
		// Write the pod spec, read its logs. Slightly more to assemble than the
		// other RBAC labs, but still short.
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "Pod Creation Escalation",
		Flag:        "k8labs{create_pods_runs_your_code_as_any_service_account}",
		Description: "You have a shell whose service account can create pods in the ops namespace but read nothing itself. A powerful build account there can read the flag Secret in production. Launch a pod that runs as that account and have it hand you the flag.",
		Hint:        "You hold `create` on `pods` in `ops` - check `kubectl auth can-i --list`. Nothing stops a pod you create from setting `serviceAccountName` to another account in the namespace, so the pod boots with that account's token. Run one as `builder` that reads the secret and prints it, then read the pod's logs. The nested cluster pulls no images, so use the image already present - `k8labs-foothold:1.31` with `imagePullPolicy: Never`.",
		Nested: &NestedSpec{
			Manifests:      createPodsManifests,
			EntryNamespace: "dev",
			EntryPod:       "foothold",
			EntryPort:      0,
			// Not a bound-SA cross-namespace read: the foothold reaches production
			// by launching a pod that wears a privileged identity.
			AttackPath: &AttackPath{From: "foothold", To: "ns:production", Label: "create pod as builder"},
		},
	},
	// RoleBinding self-grant: the foothold can create RoleBindings and holds the
	// `bind` verb on a powerful role, which defeats RBAC's escalation-prevention
	// check. It binds that role to itself and reads the flag. The lesson is the
	// check itself. See modules/rolebinding-escalate.yaml.
	"rolebinding-escalate": {
		Order: 80,
		// Discover the bind grant, write one binding, read the secret. Short, but
		// the concept (why create-rolebindings alone is not enough) is the point.
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "RoleBinding Self-Grant",
		Flag:        "k8labs{bind_verb_defeats_the_rbac_escalation_check}",
		Description: "You have a shell whose service account can create RoleBindings and read nothing. Naively binding a powerful role to yourself is blocked by Kubernetes - unless you also hold the one verb that defeats that block. Find it, grant yourself secret access, and read the flag in production.",
		Hint:        "You can `create` RoleBindings in `production` (`kubectl auth can-i --list`), but binding a role you don't already hold is normally rejected: \"attempting to grant RBAC permissions not currently held\". Look closely at your grants for the `bind` verb on a specific ClusterRole - that is the exception the API server makes. Bind that role to your own service account, then read the Secret.",
		Nested: &NestedSpec{
			Manifests:      rolebindingEscalateManifests,
			EntryNamespace: "dev",
			EntryPod:       "foothold",
			EntryPort:      0,
			// A self-grant, not a bound-SA cross-namespace read: the foothold awards
			// itself a role that reaches the production Secret.
			AttackPath: &AttackPath{From: "foothold", To: "ns:production", Label: "bind → self-grant secret-admin"},
		},
	},
	// Sidecar injection (Order 40+): the execution/persistence track. The foothold
	// can patch a Deployment in ops but reads nothing. It injects a sidecar
	// container that runs inside the workload's pods, borrowing their service
	// account token to read the production flag. See modules/sidecar-inject.yaml.
	"sidecar-inject": {
		Order: 70,
		// Patch the workload, wait for the roll, read the sidecar's logs. A little
		// more to assemble than the pure-RBAC labs.
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "Sidecar Injection",
		Flag:        "k8labs{patch_a_deployment_to_inject_a_sidecar_and_borrow_its_token}",
		Description: "You have a shell that can patch Deployments in the ops namespace but read nothing itself. A payments workload there runs as an identity that can read the flag Secret in production. Inject a container into it that hands you the flag.",
		Hint:        "You can `patch` Deployments in `ops` (`kubectl auth can-i --list`). You can't add a container to a running Pod, but you can patch the Deployment that owns it - the new pods come up with your container inside, sharing the pod's mounted token. Inject a sidecar (image `k8labs-foothold:1.31`, `imagePullPolicy: Never`) that reads the secret and prints it, then read that container's logs.",
		Nested: &NestedSpec{
			Manifests:      sidecarInjectManifests,
			EntryNamespace: "dev",
			EntryPod:       "foothold",
			EntryPort:      0,
			AttackPath:     &AttackPath{From: "foothold", To: "payments", Label: "patch → inject sidecar"},
		},
	},
	// The container-escape track (Order 21-23): breaking out of the container to
	// the node. "Winning" here compromises only the disposable nested node inside
	// the microVM - the same containment as suid-privesc.
	//
	// hostPath escape: the foothold pod mounts the node's root filesystem, so it
	// reads a flag file that lives on the node, outside any container. See
	// modules/hostpath-escape.yaml.
	"hostpath-escape": {
		Order: 90,
		// Recognise the mount, read the node file. Comparable to suid-privesc.
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "hostPath Node Escape",
		Flag:        "k8labs{hostpath_root_mount_reads_the_whole_node}",
		Description: "Your pod was carelessly given a hostPath mount of the node's root filesystem. The flag is not in the cluster - it is a root-owned file on the node itself. Use the mount to read it.",
		Hint:        "This has nothing to do with RBAC. Look at what is mounted into your pod (`mount`, or `ls /`). A hostPath mount of the node's `/` means the container boundary is gone - you can read the node's filesystem directly. The flag was planted somewhere under the node's `/opt`.",
		Nested: &NestedSpec{
			Manifests:      hostpathEscapeManifests,
			EntryNamespace: "lab",
			EntryPod:       "workstation",
			EntryPort:      0,
			// A container-to-node escape has no cross-namespace route to derive, so
			// the module names its own edge - the workstation breaking out in place,
			// drawn as a loop like suid-privesc.
			AttackPath: &AttackPath{From: "workstation", To: "workstation", Label: "hostPath → node root"},
		},
	},
	// Privileged breakout: the foothold is a privileged, hostPID pod. The student
	// breaks out to the node via /proc/1/root and reads a node file - then turns
	// to the Defender pane and watches the in-guest Tetragon sensor catch the
	// /proc/1/root dereference (Module.EBPF, one of three attack+detect labs). See
	// modules/privileged-escape.yaml.
	"privileged-escape": {
		Order: 100,
		// 45m, like ebpf-detect: there is a second half here - after the escape,
		// read the sensor's feed and find your own breakout. An eBPF session also
		// reserves 2.5 GiB vs a normal lab's 1.5 GiB (see SessionMemory), so this
		// is kept to what the two halves genuinely take, not longer.
		Timing:      &Timing{MaxAge: 45 * time.Minute},
		DisplayName: "Privileged Pod Breakout",
		Flag:        "k8labs{privileged_hostpid_proc_1_root_is_the_node}",
		Description: "Your pod is privileged and shares the host's process namespace, and an eBPF security sensor (Tetragon) is running in your lab. The flag is a root-owned file on the node. Break out of the container to read it, then watch the sensor catch you dereferencing the node's init to reach the host filesystem.",
		Hint:        "Again, not an RBAC problem - look at your pod's securityContext (`privileged`) and that it shares the host PID namespace. With hostPID, the node's init is PID 1 inside your pod, and being privileged lets you read its filesystem: `/proc/1/root` is the node's root. The flag is under the node's `/opt`. Then switch to the Defender pane - the sensor hooks the open() of any path under `/proc/1/root`, so your escape shows up there even though the shell reveals nothing.",
		EBPF:        true,
		Nested: &NestedSpec{
			Manifests:      privilegedEscapeManifests,
			EntryNamespace: "lab",
			EntryPod:       "workstation",
			EntryPort:      0,
			AttackPath:     &AttackPath{From: "workstation", To: "workstation", Label: "privileged → /proc/1/root (detected)"},
			// The Defender pane tails the in-guest Tetragon sensor's export-stdout
			// sidecar (see modules/privileged-escape.yaml).
			Defender: &DefenderSpec{Namespace: "tetragon", PodLabel: "app=tetragon", Container: "export-stdout"},
		},
	},
	// Node token harvest: the payoff of the escape track. With node filesystem
	// access, the foothold harvests a privileged pod's service-account token off
	// the node (the kubelet writes them to disk) and uses it against the API to
	// read the flag - then turns to the Defender pane and watches the in-guest
	// Tetragon sensor catch the read of another pod's token file off the node
	// (Module.EBPF, one of three attack+detect labs). See
	// modules/node-token-harvest.yaml.
	"node-token-harvest": {
		Order: 110,
		// 45m, like ebpf-detect: after the harvest and API pivot there is a second
		// half - read the sensor's feed and find the token read that gave you away.
		// An eBPF session reserves 2.5 GiB vs a normal lab's 1.5 GiB (see
		// SessionMemory), so this is kept to what the two halves genuinely take.
		Timing:      &Timing{MaxAge: 45 * time.Minute},
		DisplayName: "Node Token Harvest",
		Flag:        "k8labs{node_access_is_every_pods_token_is_cluster_access}",
		Description: "Your pod can read the node's filesystem but your own account can read nothing, and an eBPF security sensor (Tetragon) is running in your lab. The kubelet writes every pod's service-account token to disk on the node. Harvest a powerful one to read the flag, then watch the sensor catch you reading another pod's token off the node.",
		Hint:        "Your `station` account can't read the flag - check with `kubectl auth can-i`. But you can read the node: the kubelet stores every pod's token under `/var/lib/kubelet/pods/.../token`. Find them through your host mount, and use `kubectl --token=<stolen>` to find which one can read secrets - then read the `flag` Secret in `ops` with it. Then switch to the Defender pane - the sensor hooks the open() of any kubelet `token` file read through the node mount, so the harvest shows up there.",
		EBPF:        true,
		Nested: &NestedSpec{
			Manifests:      nodeTokenHarvestManifests,
			EntryNamespace: "lab",
			EntryPod:       "workstation",
			EntryPort:      0,
			AttackPath:     &AttackPath{From: "workstation", To: "ns:ops", Label: "harvest token → API (detected)"},
			// The Defender pane tails the in-guest Tetragon sensor's export-stdout
			// sidecar (see modules/node-token-harvest.yaml).
			Defender: &DefenderSpec{Namespace: "tetragon", PodLabel: "app=tetragon", Container: "export-stdout"},
		},
	},
	// Image pull secret theft (Order 45): the supply-chain lab. The foothold's SA
	// was over-granted secret read, and a dockerconfigjson imagePullSecret in the
	// namespace is a standing registry credential. The student harvests it and
	// decodes the nested base64 to recover the registry account - the foothold for
	// planting a poisoned image. Pure RBAC/Secret objects, no eBPF. See
	// modules/imagepullsecret-theft.yaml.
	"imagepullsecret-theft": {
		Order: 45,
		// Enumerate secrets, spot the pull secret, decode two layers. Short, like
		// the other read-and-decode labs.
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "Image Pull Secret Theft",
		Flag:        "k8labs{registry_pull_secret_is_a_stored_credential}",
		Description: "Your service account was granted read on Secrets in its namespace - a harmless-looking over-grant. But one of those Secrets is the cluster's imagePullSecret: a standing registry credential. Harvest it and recover the registry account's password.",
		Hint:        "List the Secrets you can read (`kubectl get secrets`). One is type `kubernetes.io/dockerconfigjson` - a registry pull credential. Read its `.dockerconfigjson` (it is base64, like all Secret data), and inside the JSON the `auth` field is itself base64 of `username:password`. Decode both layers; the password is the flag. That password is registry access, which is how a poisoned image gets in.",
		Nested: &NestedSpec{
			Manifests:      imagePullSecretTheftManifests,
			EntryNamespace: "lab",
			EntryPod:       "foothold",
			EntryPort:      0,
			AttackPath:     &AttackPath{From: "foothold", To: "ns:lab", Label: "read pull secret → registry creds"},
		},
	},
	// eBPF egress detection (Order 115): a network-hook eBPF lab. The student
	// beacons to a collector on the C2 port and the in-guest Tetragon sensor
	// catches the outbound connection via a tcp_connect kprobe - a different hook
	// class from the file/credential ones the other eBPF labs use, and the only
	// lab about network telemetry. The flag rides the egress alert (Module.EBPF).
	// See modules/ebpf-egress.yaml.
	"ebpf-egress": {
		Order: 115,
		// 45m, like the other eBPF labs: make the beacon, then read the sensor's
		// feed and find the connection it logged. An eBPF session reserves 2.5 GiB
		// vs a normal lab's 1.5 GiB (see SessionMemory).
		Timing:      &Timing{MaxAge: 45 * time.Minute},
		DisplayName: "Egress Beacon Detection",
		Flag:        "k8labs{ebpf_catches_the_c2_beacon_at_the_socket}",
		Description: "You have a foothold and want to call home. Beacon to the collector on the C2 port - but an eBPF sensor (Tetragon) hooks the kernel's tcp_connect, so the outbound connection is logged at the socket the instant you make it. Read the flag off the egress alert.",
		Hint:        "Open an outbound TCP connection to the collector on port 4444 - `nc -w2 collector.lab.svc 4444` is enough; the connection itself is the event. Then open the Defender pane: the sensor hooks tcp_connect and posts every connection to that C2 port, keyed to your process and the destination. The flag rides that alert - a process can scrub its history and exit, but it cannot un-send the SYN the kernel already saw.",
		EBPF:        true,
		Nested: &NestedSpec{
			Manifests:      ebpfEgressManifests,
			EntryNamespace: "lab",
			EntryPod:       "workstation",
			EntryPort:      0,
			AttackPath:     &AttackPath{From: "workstation", To: "ns:lab", Label: "beacon → :4444 (detected)"},
			// The Defender pane tails the in-guest Tetragon sensor's export-stdout
			// sidecar (see modules/ebpf-egress.yaml).
			Defender: &DefenderSpec{Namespace: "tetragon", PodLabel: "app=tetragon", Container: "export-stdout"},
		},
	},
	// Static pod persistence (Order 41): with a writable mount of the kubelet's
	// static-pod directory, the foothold drops a manifest the kubelet runs
	// directly - no API server, no admission - to gain node-wide access it was
	// never granted. See modules/static-pod-persist.yaml.
	"static-pod-persist": {
		Order:       120,
		Timing:      &Timing{MaxAge: 30 * time.Minute},
		DisplayName: "Static Pod Persistence",
		Flag:        "k8labs{static_pods_run_with_no_apiserver_no_admission}",
		Description: "Your pod can write to the kubelet's static-pod directory but can't read the node otherwise. The kubelet runs any manifest placed there, bypassing the API server entirely. Use that to run a pod with the access you lack and read the flag.",
		Hint:        "Look at what you can write (`ls -la /manifests`) - it's the kubelet's static-pod directory. Any pod manifest you drop there runs with NO admission control: you can request `hostPath` and `privileged` the API server would reject. Write a static pod that mounts the node's `/` and reads the flag under `/opt`, then have it drop the loot somewhere your foothold can read - the same directory works.",
		Nested: &NestedSpec{
			Manifests:      staticPodPersistManifests,
			EntryNamespace: "lab",
			EntryPod:       "workstation",
			EntryPort:      0,
			AttackPath:     &AttackPath{From: "workstation", To: "workstation", Label: "static pod → node bypass"},
		},
	},
	// Malicious admission webhook (Order 42): the persistence capstone. The
	// foothold registers a MutatingWebhookConfiguration that injects a sidecar
	// into every pod admitted in production; a lab-provided TLS webhook server
	// (its binary carried in the bundle, see modules/malicious-webhook.yaml)
	// does the injection. The injected container reads the flag with the target
	// pod's token.
	"malicious-webhook": {
		Order:       130,
		Timing:      &Timing{MaxAge: 45 * time.Minute},
		DisplayName: "Malicious Admission Webhook",
		Flag:        "k8labs{a_mutating_webhook_backdoors_every_admitted_pod}",
		Description: "You can create admission webhooks but read no secret. A mutating webhook rewrites every pod the API server admits. Register one that injects a container into production's workloads, trigger a pod, and read what your injected container captured.",
		Hint:        "You hold `create` on `mutatingwebhookconfigurations` (`kubectl auth can-i --list`) - that verb rewrites every admitted pod. A webhook server is already running at the `injector` Service in the `attacker` namespace; its CA bundle is mounted for you at `/etc/webhook-ca/ca.crt`. Register a MutatingWebhookConfiguration pointing pod CREATE at it. SCOPE IT SAFELY: a `namespaceSelector` for `production` only, and `failurePolicy: Ignore`, so you don't wedge the cluster. Then delete the `orders` pod so it re-rolls through your webhook, and read the injected `audit` container's logs.",
		Nested: &NestedSpec{
			Manifests:      maliciousWebhookManifests,
			EntryNamespace: "lab",
			EntryPod:       "workstation",
			EntryPort:      0,
			AttackPath:     &AttackPath{From: "workstation", To: "ns:production", Label: "webhook → inject sidecar"},
		},
	},

	// Kernel-driver research labs (Track: research). No planted flag - these are
	// discovery sandboxes: the microVM's own KASAN+KCOV+BTF kernel IS the target,
	// attacked from the workstation pod and watched in the Defender pane. Two
	// kernels, same nested content - dev is unhardened, hard carries production
	// mitigations. Gated to special users by simply not granting them broadly
	// (no GrantOnly field; the admin grant is the gate). See
	// docs/architecture/kasan-research-lab.md.
	"driver-research-dev": {
		Order:       200,
		Timing:      &Timing{MaxAge: 4 * time.Hour, Idle: 60 * time.Minute},
		DisplayName: "Kernel Driver Research - Dev (unhardened)",
		Description: "An on-demand kernel-research VM: KASAN+KCOV+BTF instrumentation on the guest with full toolchain ready in userspace.",
		Hint:        "The kernel itself is what you research - build and insmod the sample driver in /app/driver, run /app/fuzz to trip the UAF under KASAN, then groom a slab reclaim to overwrite cred. KASLR is off in this lab; the eBPF pane (Tetragon) gives live kernel instrumentation as you work.",
		Kernel:      KernelResearchDev,
		Nested: &NestedSpec{
			Manifests:      driverResearchManifests,
			EntryNamespace: "lab",
			EntryPod:       "workstation",
			EntryPort:      0,
			Defender:       &DefenderSpec{Namespace: "tetragon", PodLabel: "app=tetragon", Container: "export-stdout"},
		},
	},
	"driver-research-hard": {
		Order:       210,
		Timing:      &Timing{MaxAge: 4 * time.Hour, Idle: 60 * time.Minute},
		DisplayName: "Kernel Driver Research - Hardened (realistic)",
		Description: "An on-demand kernel-research VM: KASAN+KCOV+BTF instrumentation on the guest with full toolchain ready in userspace.",
		Hint:        "Same kernel, hardened. Cross-cache to beat RANDOM_KMALLOC_CACHES, leak to defeat KASLR, and prefer a data-only cred overwrite over a fragile pointer hijack. The eBPF pane gives live kernel instrumentation as you work.",
		Kernel:      KernelResearchHard,
		Nested: &NestedSpec{
			Manifests:      driverResearchManifests,
			EntryNamespace: "lab",
			EntryPod:       "workstation",
			EntryPort:      0,
			Defender:       &DefenderSpec{Namespace: "tetragon", PodLabel: "app=tetragon", Container: "export-stdout"},
		},
	},
}

// trackOf assigns every module to the tactic bucket the sidebar groups it
// under. Kept as one central map (rather than a field on each module literal)
// so the whole curriculum shape is legible in one place and a miscategorised
// lab is a one-line fix. The frontend renders tracks in the order below; Order
// sorts labs within a track.
//
// First Flight is the deliberate mixed sampler a new student clears first (an
// RBAC escape, a Linux privesc, a blue-team eBPF lab, an impersonation) before
// the tactic tracks begin. Recon then teaches the reconnaissance every later
// attack assumes. TestEveryModuleHasAKnownTrack asserts this stays exhaustive.
var trackOf = map[string]string{
	"ns-escape":      "first-flight",
	"suid-privesc":   "first-flight",
	"ebpf-detect":    "first-flight",
	"sa-impersonate": "first-flight",

	"recon-first-contact": "recon",
	"recon-who-am-i":      "recon",
	"recon-neighborhood":  "recon",

	"create-pods":          "privesc",
	"rolebinding-escalate": "privesc",

	"imagepullsecret-theft": "lateral",
	"pod-exec":              "lateral",
	"node-token-harvest":    "lateral",
	"ebpf-egress":           "lateral",

	"hostpath-escape":   "escape",
	"privileged-escape": "escape",

	"sidecar-inject":     "persistence",
	"static-pod-persist": "persistence",
	"malicious-webhook":  "persistence",

	"driver-research-dev":  "research",
	"driver-research-hard": "research",
}

// The map key is the module's identity; Slug is filled from it here so the two
// can never drift and no literal has to repeat its own name. Track is filled
// from trackOf for the same reason.
// researchVermagic is the vermagic both research guest kernels report. dev and
// hard differ in kernel CONFIG (hardening, RANDOM_KMALLOC_CACHES) but vermagic
// encodes only version + SMP + mod_unload, so the two agree - confirmed by
// building the sample driver against each live kernel (2026-09-10).
//
// Pinned here beside the catalog because it is derived from the kernel version:
// bump it whenever services/session-runner/research/Dockerfile's KVER and
// infra/cluster/kata-fc-research-node.sh move. A stale value fails LOUD - every
// driver reads as mismatched in the picker - rather than silently passing a
// module that cannot load.
const researchVermagic = "6.18.35 SMP mod_unload"

func init() {
	for slug, m := range modules {
		m.Slug = slug
		m.Track = trackOf[slug]
		m.HasDefender = m.Nested != nil && m.Nested.Defender != nil
		if m.IsResearch() {
			m.ExpectedVermagic = researchVermagic
		}
		modules[slug] = m
	}
}

func GetModule(slug string) (Module, bool) {
	m, ok := modules[slug]
	return m, ok
}

// ListModules returns the catalog in a stable, deterministic order.
//
// The sort is load-bearing, not cosmetic: `modules` is a map, and Go randomises
// map iteration on purpose, so this used to hand back a different order on every
// call. The sidebar renders the array as-is, so its lab list visibly reshuffled
// whenever the modules query refetched - which picking a lab is enough to do.
// Ties break on Slug so the order is total even if two modules share an Order
// (or a new one forgets to set it), never falling back to map order.
func ListModules() []Module {
	out := make([]Module, 0, len(modules))
	for _, m := range modules {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].Slug < out[j].Slug
	})
	return out
}
