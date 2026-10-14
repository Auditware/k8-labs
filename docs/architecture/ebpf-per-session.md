# eBPF per-user-session - design & rollout plan

Status: **approved, in build** (2026-07). Gated technical risk is retired - see
"PoC facts". Scope decisions locked (2026-07-23) - see "Locked decisions".

## Locked decisions (2026-07-23)

The five open decisions are resolved; this section is the authority, the older
"Open decisions for the owner" list below is kept only for the reasoning.

1. **RuntimeClass, not annotation.** Capability is expressed by a dedicated
   `kata-fc-ebpf` RuntimeClass the backend selects per module. Default `kata-fc`
   stays byte-for-byte as today.
2. **Ship on the interim `-debug` kernel; lean fragment is a fast-follow.** The
   `-debug` artifact is already installed and PoC-proven, so Phase 1 is zero
   build. The lean 6-flag kernel (below) lands before eBPF is more than an opt-in
   demo. Note the interim kernel *is* the output of Kata's `KERNEL_DEBUG_ENABLED=yes`
   build (`tools/packaging/kernel` README: that flag enables "debug and eBPF
   configs - BTF, kprobes, ftrace, BPF"); the lean build is the same tool with a
   minimal config fragment instead of the whole debug bundle (365 MB -> ~40 MB).
3. **Tooling: Tetragon in-guest** (k8s-aware, closest to the real product's own
   EDR). bpftrace/bpftool may ride along in the image but the first module and the
   defender view are Tetragon-shaped.
4. **Defender view: yes, as a third right-pane tab** (Terminal | Topology |
   Defender). See the corrected data path below - it does NOT go through Loki.
5. **Kernel maintenance owner + cadence:** still open, tracked in Phase 6; does
   not block Slices A-C (local) or the interim-kernel prod bring-up.

### Correction: the defender view does NOT use the Alloy->Loki pipeline

The "Optional feature layer" section below assumed the in-guest sensor writes to
the *runner* pod's stdout, which host Alloy would ship to Loki. That is wrong for
this architecture. Tetragon-in-guest runs as a pod *inside the nested k3s*, so its
output is a **nested** pod log - and per the process-creds-changed comment, nested
activity never reaches outer Alloy/Loki (nested processes are attributed to
namespace "" and dropped by the export-denylist). The backend already holds the
nested kubeconfig and already bridges nested pod I/O to a browser websocket
(`internal/labs/terminal.go` `nestedTerminal`). So the defender pane **tails the
nested Tetragon pod's logs via the nested kube API and streams to a third
websocket** - the exact terminal-bridge pattern, no new sink, no Loki dependency.

### Build slices (A-C are local-testable under runc, D is the only prod gate)

- **Slice A - backend runtime selection.** `Module.EBPF bool`; a second config
  value `LABS_RUNTIME_CLASS_EBPF` -> `kata-fc-ebpf`; `buildRunnerPod` selects it
  when the module is eBPF. The single kernel-selection point. Locally both runtime
  classes are empty (runc, whose host kernel already has BTF), so an eBPF lab runs
  with no custom kernel - which is why A-C need no prod work.
- **Slice B - eBPF lab image + module.** Tetragon baked into an eBPF foothold
  image (added to the airgap bake so nested pulls stay local); the nested manifest
  bundle runs Tetragon in the nested cluster + grants the lab pod `CAP_BPF`/
  `CAP_PERFMON`/`CAP_SYS_ADMIN` + `bpffs`/`tracefs`/`/sys/kernel/btf`, under the
  nested cluster's own PSA; first eBPF course under `internal/course/content`.
- **Slice C - defender pane.** Backend `nestedDefender` bridge (tail nested
  Tetragon pod via the nested kubeconfig -> ws, mirrors `terminal.go`); frontend
  third tab reusing the `NestedTerminal.tsx` stream plumbing.
- **Slice D - prod kernel + node config.** Point `configuration-fc-ebpf.toml`
  `kernel =` at the installed `-debug` kernel; `kata-fc-ebpf` containerd handler +
  one containerd restart; `RuntimeClass kata-fc-ebpf` under `apps/labs`; node
  bootstrap script. Then the Phase-4 gate (real CO-RE sensor loads in-guest).

### Build status (2026-07-23)

**Slice A - DONE, tested.** `Module.EBPF`, `LABS_RUNTIME_CLASS_EBPF`,
`runtimeClassFor`, `buildRunnerPod` selection; chart wiring; unit tests (eBPF ->
kata-fc-ebpf, non-eBPF -> kata-fc, local -> runc fallback).

**Slice B - built, sensor design validated live on k3d.** Built: `ebpf-detect`
module bundle (`internal/labs/modules/ebpf-detect.yaml` - a standalone
Tetragon DaemonSet with agent + `tetra getevents` stdout sidecar, a
`--tracing-policy-dir` commit_creds `TracingPolicy`, and a SUID-busybox trigger
in the workstation foothold); registry entry (`EBPF: true`); runner resource
sizing (`runnerResources`, 1.5 Gi request for eBPF = the concurrency lever) +
`SESSION_EBPF=1`; `entrypoint.sh` gated tetragon.tar import; airgap bake
(Tiltfile + CI) for `quay.io/cilium/tetragon:v1.7.0`.

Proven end-to-end on k3d (host kernel 6.10.14-linuxkit) by running the exact
sensor design in a throwaway namespace ALONGSIDE the platform Tetragon:
- Standalone Tetragon (no operator/CRD) loads eBPF via `--tracing-policy-dir`
  with `--enable-k8s-api=false`; BTF discovered, commit_creds kprobe attached.
- **Isolated bpffs works** (decision: per-pod bpffs) - `emptyDir` at `/sys/fs/bpf`
  + `mount -t bpf` in the agent command, so its pinned maps never touch the host
  `/sys/fs/bpf/tetragon`; both sensors ran with zero collision. Same model in
  prod (each microVM already isolated) and locally, so concurrent eBPF sessions
  coexist too.
- **The detection fires:** an unprivileged user exec'ing the SUID-root binary
  produced a `commit_creds` event `binary=/shared/suid-demo uid=0`, streamed as
  JSON through the `tetra getevents` sidecar - the exact feed Slice C's Defender
  pane will tail.
- Bug found + fixed live: a plain ConfigMap mount makes `--tracing-policy-dir`
  load the policy twice (via k8s's `..data` symlink) -> agent exits; `subPath`
  projects only the real file. Applied to the module manifest.

**Full nested flow proven end-to-end on k3d (2026-07-23).** Built the eBPF runner
image (tetragon.tar baked, 80 MB; runner image 338 -> 503 MB), booted an
`SESSION_EBPF=1` runner (poc-pod shape), applied `ebpf-detect` INTO its nested
k3s, and confirmed the sensor works one level deeper: nested Tetragon loaded eBPF
(BTF, commit_creds kprobe, "Listening for events"), and a student escalation
(`backup-tool "x; id"`) produced the full detection narrative in the export-stdout
feed - `process_exec` of backup-tool AND a `commit_creds` event carrying
`process_credentials_arg{uid:0,euid:0,suid:0}` + root caps. The escalation really
reaches root (read the root-only flag).

Three real bugs found and fixed in the nested run (none visible on the outer
cluster):
1. `mountPropagation: Bidirectional` on a `/sys/kernel/debug` hostPath fails on
   the nested node ("mounted on /sys but not a shared mount"). Fix: the agent
   self-mounts debugfs/tracefs in its own mount ns (same pattern as the bpffs
   isolation) - also more robust for the Kata guest.
2. A plain ConfigMap mount makes `--tracing-policy-dir` load the policy twice
   (via k8s `..data` symlink) -> agent exits. Fix: `subPath`.
3. busybox DROPS SUID for non-suid applets (a busybox security feature), so a
   SUID busybox never actually reaches euid 0. Fix: reuse suid-privesc's real
   compiled `backup-tool` (which keeps root) - and its exec is exactly what the
   platform's own commit_creds policy documents catching.

Attribution nuance (inherent, not a defect): the `commit_creds` event's process
`binary` is "unknown" for a SUID exec because commit_creds fires mid-execve before
the new binary is cached - the platform's host policy has the same behavior. The
signal is the captured cred payload (euid 0 + root caps), and `process_exec` gives
the binary correlation, so the Defender pane (Slice C) has everything it needs.

Remaining: the prod Kata `-debug` guest kernel (Slice D / Phase-4 gate) - the only
piece the local runc run can't exercise.

**Slice C (Defender pane) - built.** Backend `nestedDefender` bridge
(`internal/labs/defender.go`): resolves the sensor DaemonSet pod by label
(`Module.Nested.Defender` = ns `tetragon`, `app=tetragon`, container
`export-stdout`), follows its container log via the nested kubeconfig, and pumps
each JSON line to a read-only websocket - the terminal bridge's read-only cousin,
same keepalive/ownership model, no Loki. Route
`GET /api/v1/sessions/{id}/defender` (auth + ownership, 400 for non-eBPF).
Frontend: a third right-pane tab (`DefenderPane.tsx`), shown only for
`module.ebpf`, that parses the feed and highlights the `commit_creds` euid-0
escalation (leading with the credential fact since the SUID-exec binary is often
"unknown"). Verified: backend builds/vets/`-race` tests pass (pod-resolution via
fake clientset); frontend builds + 28 tests pass, including 6 parse tests over the
REAL event JSON captured from the nested run. Rendered the real `DefenderPane`
component fed by the real captured events (stubbed socket) and screenshotted it:
`commit_creds` euid-0 rows highlight amber with ⚠, exec rows stay quiet - the
intended UX. A full-session in-browser render is gated only by OAuth login (can't
be automated); the feed, parse logic, and rendering are each proven.

### Phase-4 gate result (2026-07-23) - interim -debug kernel is NOT sufficient

Deployed to prod end to end (backend/frontend/session-runner images via CI on
main; kata-fc-ebpf containerd handler + configuration-fc-ebpf.toml pointing at
`vmlinux-debug.container` on the AX41; kata-fc-ebpf RuntimeClass). A runner pod
under `kata-fc-ebpf` **boots on the AX41 in ~18s** (the -debug microVM works), the
nested k3s comes up, and SESSION_EBPF=1 imports tetragon.tar. eBPF itself works
in-guest: `BTF discovery: default kernel btf file found`, BPF programs compile,
CO-RE features detected.

**But Tetragon's `__base__` sensor crashloops on the -debug kernel:**
```
kprobe_multi: false
sensor __base__ failed to load: attaching 'event_exit_acct_process' failed:
  creating perf_kprobe PMU (arch-specific fallback for "acct_process"):
  token __x64_acct_process: not found
```
The -debug kernel lacks `kprobe_multi` (needs `CONFIG_FPROBE`), so Tetragon falls
back to per-symbol perf-kprobe attach, which then can't resolve `acct_process`
(kernel also lacks `CONFIG_BSD_PROCESS_ACCT`). So the "interim: ship on the shipped
-debug kernel" plan does NOT hold for a full CO-RE Tetragon sensor - it only ever
proved a single bpftrace-style commit_creds kprobe, not the base sensor. The lean
purpose-built kernel is REQUIRED, not optional. Local runc masked this (host
kernels have kprobe_multi).

### Phase-4 gate PASSED (2026-07-23) - the full recipe

Built the kernel on the AX41 and iterated the gate to green. The complete recipe
(more than the plan's original flag list):

1. **Kernel** = the shipped -debug config (BTF + kprobes + tracefs) + two flags:
   `CONFIG_FPROBE=y` (kprobe_multi) and `CONFIG_BSD_PROCESS_ACCT=y` (acct_process).
   Built with Kata's `build-kernel.sh` (`KERNEL_DEBUG_ENABLED=yes setup`, then
   `scripts/config -e FPROBE -e BSD_PROCESS_ACCT`, `make olddefconfig`, build).
2. **BTF gotcha:** `olddefconfig` SILENTLY disables `CONFIG_DEBUG_INFO_BTF` if
   `pahole` (pkg `dwarves`) is absent at config time (`CONFIG_PAHOLE_VERSION=0`).
   Install `dwarves` FIRST, then re-enable BTF and rebuild, or the kernel ships
   with no `.BTF` section and the sensor aborts "kernel not in supported list".
3. **Runtime sysctls (in-guest):** the sensor reads `/proc/kallsyms`; the guest
   boots with `kptr_restrict=1` + `perf_event_paranoid=2`, so every address reads
   0 and the sensor aborts "no kernel symbols found". Need BOTH `kptr_restrict=0`
   AND `perf_event_paranoid<=1`. Set in `entrypoint.sh`, gated on `SESSION_EBPF=1`
   (guest-global, gone with the microVM, normal labs keep the tighter defaults).

Result on the AX41 Kata/Firecracker microVM: `Loaded generic kprobe sensor ->
kprobe_multi`, `Added TracingPolicy with success`, `Listening for events`, and
58 `commit_creds` events captured in-guest. eBPF-in-microVM works end to end.

**Slice D - DONE and gate-verified on prod.** Custom kernel built + installed on
the AX41 (`vmlinux-6.18.35-197-ebpf`), `configuration-fc-ebpf.toml` repointed,
`kata-fc-ebpf` RuntimeClass live, entrypoint sysctl fix in code (ships via CI in
`session-runner:latest`). Remaining ops: fold the kernel build into a repeatable
node bootstrap (it was done by hand on the AX41), and the lean/minimal kernel
(vs the ~370 MB debug-derived one) as a later size optimization.
Built: `infra/runtimeclass/kata-fc-ebpf.yaml` (RuntimeClass mirroring kata-fc); the
`kata-fc-ebpf` node-config step in `infra/cluster/kubeadm-2node.md` (interim -debug
kernel, `configuration-fc-ebpf.toml`, a second containerd handler reusing
devmapper, one containerd restart); chart default `labsRuntimeClassEbpf: kata-fc-ebpf`
(Tilt overrides to empty so local stays runc). Helm renders it, RuntimeClass
validates. Remaining (needs the AX41, a human deploy step - like kata-fc itself):
install the kernel + handler on the labs node, `kubectl apply` the RuntimeClass,
then the Phase-4 gate (a real CO-RE sensor loads in-guest on the Kata -debug
kernel - the last thing local runc can't exercise).

## Goal

Let selected lab sessions run on an eBPF-capable guest kernel so a student can
run eBPF tooling (Tetragon, bpftrace, bpftool) and learn security monitoring
*inside their own lab microVM*. This is a student-facing capability - the eBPF
is the product - not admin surveillance (that distinction is why it is worth the
kernel cost here, unlike the boundary-EDR discussion in
`apps/tetragon/templates/process-creds-changed.yaml`).

Design principle: **per-session opt-in**. Only eBPF-lab sessions pay the heavier
kernel; every normal lab keeps the lean, fast-booting stock kernel and its
minimal attack surface.

## PoC facts (what is already proven, 2026-07)

Verified on the prod AX41 labs node by booting a Kata microVM on the shipped
`-debug` guest kernel:

- The Kata boundary is NOT the blocker - only the stripped default kernel was.
- Stock `kata-fc` guest kernel (`vmlinux-6.18.35-197`): no BTF, no tracefs, no
  kprobes. eBPF cannot load.
- Shipped `-debug` variant of the *same* kernel: boots under Firecracker in ~8s,
  BTF present (5.5 MB), tracefs mounted, 44,361 kprobe-able functions, and a live
  kprobe on `commit_creds` fired 17x with real trace output.
- Kata config knobs found: `enable_annotations` (kata side) is NOT sufficient on
  its own - containerd strips `io.katacontainers.*` pod annotations unless the
  runtime handler sets `pod_annotations = ["io.katacontainers.*"]`. A dedicated
  RuntimeClass avoids the annotation path entirely.

Remaining unproven (small): a full Tetragon CO-RE sensor *loading* (the PoC image
had no eBPF tooling). Very likely given BTF+kprobes+tracefs are all present;
closed in Phase 4 by running a real sensor on the eBPF image.

## Architecture decision: dedicated RuntimeClass (recommended)

Two ways to give a session a different kernel:

1. **Dedicated RuntimeClass `kata-fc-ebpf`** (recommended). Its own
   `configuration-fc-ebpf.toml` points `kernel =` at the eBPF kernel. The backend
   selects the runtime per module. Clean separation, no loosening of the default
   runtime's annotation surface, trivial to reason about ("this class = eBPF
   kernel"), and the default `kata-fc` stays byte-for-byte as today.
2. Per-pod kernel annotation. More flexible but requires enabling
   `enable_annotations += "kernel"` AND containerd `pod_annotations` passthrough
   (a containerd restart), and it broadens what any pod can request (an arbitrary
   host kernel path). Same containerd change, weaker security posture.

Go with #1. The annotation surface stays closed; capability is expressed by
runtime class, which the backend already controls per pod.

## The kernel

Two options, pick per how fast you want to ship:

- **Interim (fast):** point `configuration-fc-ebpf.toml` at the already-shipped
  `vmlinux-debug.container`. Zero build. Cost: 365 MB vs 40 MB, heavier boot and
  memory - acceptable for a first eBPF module behind an opt-in class, wrong as a
  permanent answer.
- **Production (lean):** build a kernel = stock kata config + only the tracing
  flags: `CONFIG_KPROBES`, `CONFIG_FTRACE`, `CONFIG_FUNCTION_TRACER`,
  `CONFIG_KPROBE_EVENTS`, `CONFIG_BPF_EVENTS`, `CONFIG_DEBUG_INFO_BTF`. Built via
  Kata's `tools/packaging/kernel/build-kernel.sh` (setup/build/install). The
  `-debug` variant proves these compile and boot on this exact 6.18.35 kernel, so
  it is a config delta on a known-good base, not a research project.

Maintenance is the real ongoing cost: the config delta is out-of-tree, so it must
be forward-ported on every Kata kernel/version bump and rebuilt on kernel CVEs.
Own this deliberately (see Phase 6 runbook).

## Backend / platform changes

- **Module schema** (`internal/labs/modules/*.yaml`, the `Module` struct): add an
  opt-in flag, e.g. `ebpf: true` (mirrors how `SecurityLevel` already drives
  `NamespaceFor` in `config.go`).
- **`runner.go` `buildRunnerPod`**: when the module is eBPF-enabled, set
  `runtimeClassName: kata-fc-ebpf` instead of `kata-fc`, and add the caps the
  in-guest sensor needs (`CAP_BPF`, `CAP_PERFMON`, `CAP_SYS_ADMIN`) plus the
  mounts (`bpffs` on `/sys/fs/bpf`, `tracefs`/`debugfs`, `/sys/kernel/btf`
  readable). Keep everything else identical.
- **Resource sizing**: the eBPF kernel (esp. the interim debug one) is heavier.
  The 64 GB AX41 bounds concurrency; give eBPF sessions a
  lower parallel budget / higher per-session memory reservation.
- **Nested pod**: the student runs eBPF from a pod *inside* the nested k3s, which
  shares the runner microVM's guest kernel - so it inherits the eBPF-capable
  kernel automatically. Ensure the lab's nested pod securityContext (governed by
  the nested cluster's PodSecurity, which the lab content sets) grants the caps
  and that bpffs/tracefs/btf are visible in the nested pod.
- **eBPF lab image**: bake `bpftrace`, `bpftool`, `libbpf`, kernel BTF, and
  optionally Tetragon into an eBPF-lab foothold image (separate from the tiny
  default foothold; size is fine because it is opt-in).

## Infra / GitOps changes (labs node, AX41)

- Install the eBPF kernel artifact under `/opt/kata/share/kata-containers/`.
- Add a containerd runtime handler `kata-fc-ebpf` in `/etc/containerd/config.toml`
  with `ConfigPath = .../configuration-fc-ebpf.toml`; restart containerd once
  (planned - running containers survive; brief CRI unavailability).
- Create k8s `RuntimeClass kata-fc-ebpf` (handler `kata-fc-ebpf`), via a manifest
  under `apps/labs`.
- This is bare-metal Robot config, outside Terraform (like the Robot firewall
  itself). Capture it in a node bootstrap script + doc, not click-ops
  only, so it survives a node rebuild.

## Local dev is easy (de-risks the whole thing)

Locally under runc/k3d the host kernel already has BTF, and nested pods share that
host kernel - so eBPF labs run locally with NO custom kernel. That means the
backend flag, the eBPF image, and the course content (Phases 3-5) can be built and
tested entirely locally first. Only the kernel + infra (Phases 1-2) gate prod.

## Security review

- Giving students `CAP_BPF`/`CAP_SYS_ADMIN` and a richer kernel widens what they
  can do *inside* the microVM. That is contained by Wall 1 (the microVM) - the
  blast radius is still the throwaway VM, which is the entire point of the Kata
  design. Isolation *between* eBPF sessions is unchanged (separate microVMs).
- A richer guest kernel has more guest-kernel code, but Firecracker's escape
  boundary is the jailer + minimal device model, not guest kernel size, so this
  does not directly weaken the escape boundary. Note it, do not block on it.
- Keep the eBPF kernel strictly behind the opt-in RuntimeClass so default labs
  retain the minimal surface and fast boot.
- NetworkPolicy/egress (Wall 2) unchanged.

## Student "defender view" (in scope - see corrected data path)

**Superseded by the "Correction" under Locked decisions - read that first.** The
description below is kept for context but its data path is wrong for the nested
architecture: the in-guest sensor's output is a *nested* pod log and never reaches
outer Alloy/Loki. The actual implementation (Slice C) tails the nested Tetragon
pod via the nested kubeconfig and streams to a third websocket, mirroring
`terminal.go` - no Loki dependency.

~~If students should SEE their own eBPF/Tetragon output: the in-guest sensor writes
to stdout, which the existing host Alloy already ships to Loki tagged by the
runner pod's `session`/`module`/`owner` labels (`runner.go` `runnerLabels`). Surface
it as a second pane in the lab UI or a session-scoped Grafana/embedded view.
Reuses the whole existing pipeline; no new sink.~~

## Phased rollout

- **Phase 0 - PoC (DONE):** eBPF works on the debug kernel in Firecracker. Facts
  above.
- **Phase 1 - kernel:** lean eBPF kernel build + artifact + version pin + CI. (Or
  interim: adopt the shipped `-debug` kernel to ship faster.)
- **Phase 2 - infra:** `kata-fc-ebpf` RuntimeClass + containerd handler + config
  on AX41; node bootstrap script + doc.
- **Phase 3 - backend:** module `ebpf` flag; `buildRunnerPod` runtimeClass/caps/
  mounts; resource sizing. Build/test locally (runc) first.
- **Phase 4 - image:** eBPF-lab foothold image (bpftrace/bpftool/Tetragon);
  confirm a real CO-RE sensor loads in-guest (closes the last PoC gap).
- **Phase 5 - content:** first eBPF course module + optional defender-view UI.
- **Phase 6 - harden:** security sign-off, capacity test under eBPF sessions,
  kernel-maintenance runbook (forward-port cadence, CVE rebuild).

## Open decisions for the owner

1. RuntimeClass vs annotation - recommend RuntimeClass.
2. Lean custom kernel now, or ship on the interim debug kernel and swap later.
3. Tooling students get: Tetragon (rich, k8s-aware), bpftrace (light, scriptable),
   or both.
4. Defender-view UI in scope, or terminal-based eBPF tooling only.
5. Who owns kernel forward-port + CVE rebuilds, and at what cadence.

## Rough cost

- Eng: ~2-3 weeks across phases with a lean kernel; less if starting on the debug
  kernel. Phases 3-5 are local-testable, so prod exposure is small.
- Runtime: heavier kernel per eBPF session (boot + memory) - constrain eBPF
  session concurrency on the AX41.
- Ongoing: out-of-tree kernel config forward-port + CVE rebuilds.
