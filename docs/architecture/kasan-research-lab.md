# Kernel driver research lab - design & rollout plan

Status: **LIVE in prod, acceptance passed** (updated 2026-09-02). Direct sibling
of the `kata-fc-ebpf` work - it reuses the same "custom guest kernel per session,
selected by a Module flag, admitted via the RuntimeClass allowlist" machinery.

**Both labs verified end-to-end in production** (`driver-research-dev` and
`-hard`): a session boots the KASAN guest kernel, the researcher builds and
`insmod`s the planted-UAF driver in-guest, `./fuzz uaf` trips a clean KASAN
`slab-use-after-free` report (naming the object + `kmalloc-64` cache), and the
Defender pane streams the `commit_creds` transitions. The `-hard` kernel carries
all four mitigations (KASLR, hardened freelist, randomized kmalloc caches, static
usermodehelper).

**Locked decisions:** kernels built from **Kata's shipped guest version + KASAN**
(mirror `kata-fc-ebpf`, not a 6.12 pin); **`CONFIG_CFI_CLANG` deferred** (needs a
clang build) - the four gcc-compatible mitigations ship first.

**Five prod-only bugs the local (runc) path could not surface, all fixed +
deployed** (P2 acceptance earned its keep):
1. the **runner** needs `SYS_MODULE` in its bounding set or the workstation can't
   even start (`runner.go`);
2. the **kernel build tree** must ship in-guest for `make` - baked into the (now
   glibc/Debian) research image in CI via `build-kernel.sh setup` + the captured
   `kernel-6.18.35.config` + `make modules_prepare` (`TREE_MODE` gates it off
   locally);
3. that tree has no `Module.symvers`, so the driver Makefile passes
   `KBUILD_MODPOST_WARN=1` (MODVERSIONS is off, symbols resolve at `insmod`);
4. the **workstation runs privileged** - the device cgroup blocks opening the
   driver's char device otherwise;
5. this `/dev` has no auto-population, so `load.sh` `mknod`s the node, and the
   `uaf` demo trips the bug with a use-after-free **write** (non-fatal) rather
   than calling the dangling pointer and panicking the microVM.

The frontend research labs also get a one-page visual guide (exploit-flow diagram
+ concise text, `ResearchGuide.tsx`) in place of a guided course.

## Locked decisions

1. **Two labs, split by mitigation profile, both x86_64.** `driver-research-dev`
   (mitigations off, for landing a technique) and `driver-research-hard`
   (production-grade mitigations on, to prove exploitability survives). Same
   6.18.35 base (Kata's shipped guest kernel), same instrumentation; they differ
   only in compiled-in hardening. The mitigation axis - not kernel version - is
   where exploitation difficulty lives.
2. **Guest-kernel-as-target, no nested QEMU.** The session's own microVM kernel is
   the KASAN+KCOV instrumented target, attacked directly from inside - identical to
   how `ebpf-detect` runs eBPF against the guest kernel. No inner VM, no image to
   fetch. Firecracker does not expose nested KVM to its guest, so L2 nesting is not
   an option anyway (`nested-lab-rewrite.md`).
3. **RuntimeClass, not annotation.** Two dedicated classes `kata-fc-research-dev`
   and `kata-fc-research-hard`, selected per module by a `Kernel` field. Default
   `kata-fc` and existing `kata-fc-ebpf` stay byte-for-byte.
4. **eBPF (BTF + kprobes + Tetragon) in both.** BTF gives struct offsets for the
   exploitation step; kprobes+Tetragon power the Defender tab so a researcher can
   watch the bug get weaponized live. It does not weaken the hardened lab's realism
   because eBPF is the researcher's observability lens, not part of the exploit -
   the exploit still faces the full mitigation stack.
5. **Grant-only, no flag, airgapped.** These are `Track: research`, admin-granted to
   special users (which also caps the heavy KASAN seats), have no planted flag
   (discovery, not capture-the-flag), and keep the default-deny NetworkPolicy - no
   egress needed.

   Note that grant-only gates the *catalog*, not the *content*. Until 2026-09-10 the
   shared runner image left `research.tar` at `/opt/airgap/` inside **every**
   session's microVM (the `SESSION_RESEARCH` split gated the nested-cluster import,
   not the tar's presence), while the escape modules (`hostpath-escape`,
   `privileged-escape`, `node-token-harvest`) exist to break a nested pod out to the
   nested node - which is that same filesystem. Anyone who cleared the escape track
   could therefore unpack this lab before release, and a customer did. `entrypoint.sh`
   now drops `/opt/airgap` once the tars a session actually needs are staged in
   `agent/images/`. Isolation was never affected (it is all inside the microVM,
   nowhere near the host); what leaked was the confidentiality of unreleased content.
   Any future unreleased lab baked into the shared airgap inherits this, so keep the
   removal unconditional.
6. **x86_64 only. No arm64, no new Hetzner box.** The AX41 is x86; native-speed
   arm64 needs ARM metal (a purchase) and emulated arm64 (QEMU-TCG) is the slow path
   this platform exists to avoid. arm64 is a scope decision (Android/mobile, PAC/MTE),
   not a completeness gap - see "Arch scope".
7. **Skills are out of scope.** The lab ships the prepared *environment* the
   exploitation work needs (instrumented kernel-as-target, toolchain, a starter
   driver, the Defender view). The method - and whether it is human- or
   agent-driven - is not part of this plan. Human-in-the-loop is assumed, which is
   why none of the autonomy machinery (heartbeat, unbounded TTL, result store,
   cross-session memory) is built: the human is the memory and the save.

## Goal

A throwaway microVM whose own kernel is deliberately exploitable-under-observation:
KASAN as the crash oracle, KCOV for coverage, BTF for struct introspection, kprobes
for tracing. A researcher opens a session, loads a driver, fuzzes its ioctl surface,
and drives a memory-safety bug to `uid 0` - all inside the disposable VM, watching it
through the Defender tab. The dev lab is where a technique is landed; the hardened lab
is where it is proven to survive real cloud hardening.

## What each session provides

Baked into the runner image (airgap tars, exactly like `foothold`/`tetragon`):

- the **instrumented guest kernel as the microVM kernel** - the target itself
  (the two research kernels, prod-only; P1);
- the **research workstation image** (`k8labs-research`, built from
  `services/session-runner/research/`, imported in-guest on `SESSION_RESEARCH=1`):
  **toolchain** `gcc`, `gdb`, `pahole`, `strace`, `ripgrep`, `kmod`; a **planted-UAF
  target driver** (`/app/driver/vuln.c` + `Makefile`); a **starter ioctl fuzzer**
  (`/app/fuzz/fuzz.c`); and a **`/app/README`** orientation. (Full `syzkaller` is
  not shipped - the starter fuzzer stands in; adding it is a fast-follow.)
- the **Defender tab** lit up (Tetragon in-guest, from the BTF/kprobes kernel).

No skills, no flag, no course capstone. The session is a prepared sandbox, not a
guided challenge - the frontend hides the Topology tab and shows a "Research
sandbox" orientation panel in place of the guided course.

## The two labs

Both are `Track: research`, `GrantOnly: true`, `Flag: ""`, `NeedsEgress: false`,
`Timing{MaxAge: 4h, Idle: 60m}`. The hard cap is deliberately short so an idle
or abandoned research VM frees compute fast; an actively-worked session extends it
on demand in 2h steps up to a 48h ceiling (see "Session extension" below), and the
idle clock is what usually bites first (see "What keeps a session alive"). They
differ only in `Kernel`:

```go
// internal/labs/registry.go
"driver-research-dev": {
    Slug:        "driver-research-dev",
    DisplayName: "Kernel Driver Research - Dev (unhardened)",
    Description: "Instrumented x86_64 guest kernel as the target, mitigations off for technique bring-up. Load a driver, fuzz its ioctl surface, drive a memory-safety bug to uid 0, and watch it in Defender.",
    Kernel:      KernelResearchDev,   // -> kata-fc-research-dev
    Track:       TrackResearch,
    Flag:        "",
    NeedsEgress: false,
    Timing:      Timing{MaxAge: 4 * time.Hour, Idle: 60 * time.Minute},
    GrantOnly:   true,
    Order:       1,
},
"driver-research-hard": {
    Slug:        "driver-research-hard",
    DisplayName: "Kernel Driver Research - Hardened (realistic)",
    Description: "Same instrumented sandbox with production-grade mitigations on (KASLR, randomized kmalloc caches, hardened freelist, STATIC_USERMODEHELPER; CFI_CLANG deferred - see P3). Validate whether a bug is still exploitable under real cloud hardening.",
    Kernel:      KernelResearchHard,  // -> kata-fc-research-hard
    Track:       TrackResearch,
    Flag:        "",
    NeedsEgress: false,
    Timing:      Timing{MaxAge: 4 * time.Hour, Idle: 60 * time.Minute},
    GrantOnly:   true,
    Order:       2,
},
```

## The kernel

One combined config, built twice with a different mitigation fragment. Base: linux
**6.18.35** (Kata's shipped guest kernel), x86_64 (swappable later - see "Arch scope").

Shared instrumentation (both profiles):
- `CONFIG_KASAN=y`, `CONFIG_KASAN_INLINE=y` - the crash oracle.
- `CONFIG_KCOV=y`, `CONFIG_KCOV_INSTRUMENT_ALL=y` - coverage feedback.
- `CONFIG_DEBUG_INFO_BTF=y` - struct offsets for exploitation (no full vmlinux needed).
- kprobes / fprobe / tracefs (reuse the `ebpf-per-session.md` recipe: `CONFIG_FPROBE`,
  `CONFIG_BSD_PROCESS_ACCT`, BTF, kprobes, ftrace) - Defender/Tetragon substrate.
- SysV IPC / user-ns / configfs - the reclaim-object surface (`msg_msg`,
  `pipe_buffer`), loadable modules for `insmod`ing the target driver.

Mitigation fragments (the only difference):
- **dev**: `# CONFIG_RANDOMIZE_BASE is not set` (KASLR off),
  `# CONFIG_SLAB_FREELIST_HARDENED is not set`, `# CONFIG_RANDOM_KMALLOC_CACHES is not set`,
  `init_on_alloc=0` at boot.
- **hard**: `CONFIG_RANDOMIZE_BASE=y`, `CONFIG_SLAB_FREELIST_HARDENED=y`,
  `CONFIG_RANDOM_KMALLOC_CACHES=y`, `CONFIG_STATIC_USERMODEHELPER=y`,
  `init_on_alloc=1`. (`CONFIG_CFI_CLANG` is deferred - it needs a clang build; the
  four gcc-compatible mitigations ship first. See P3 in the rollout section.)

Weight: KASAN roughly doubles RAM, and the eBPF bundle adds the ~880 MiB measured for
`kata-fc-ebpf`. Budget both profiles at the eBPF tier or higher for `SessionMemory`;
measure on first bring-up. Grant-gating keeps concurrency low enough that this fits
inside the existing node budget (see "Capacity & cost").

## Backend / platform changes

Generalize the existing `EBPF bool` into a kernel selector (small, mechanical):

```go
// internal/labs/config.go
type KernelKind string
const (
    KernelDefault      KernelKind = ""              // kata-fc
    KernelEBPF         KernelKind = "ebpf"          // kata-fc-ebpf (existing)
    KernelResearchDev  KernelKind = "research-dev"  // kata-fc-research-dev
    KernelResearchHard KernelKind = "research-hard" // kata-fc-research-hard
)
```

- `runtimeClassFor(kernel)` maps each to its class name (add
  `LABS_RUNTIME_CLASS_RESEARCH_DEV` / `..._HARD` to `apps/backend/values.yaml`,
  mirroring `labsRuntimeClassEbpf`). `buildRunnerPod` sets `runtimeClassName`.
- `SessionMemory` gains the research tier(s).
- New `Track: research` and a `GrantOnly` shape (or reuse the existing explicit-grant
  path in `progression.go` / `adminGrantLab`) so the labs are admin-granted only.
- **AI grant TTL**: extend beyond the tutor's 15 min (`ai.go aiGrantTTL`) for research
  sessions - a fuzzing window is longer than a hint. Everything else about the grant
  (revocable, exec-on-visible-terminal, no lifecycle/flag control) is reused as-is.

UI is one branch: group the `research` track, and **skip the flag capstone** for
research modules (no planted flag). Terminal / Topology / Defender / AI grant are
reused unchanged. Grant them from the admin pane exactly like any explicit-grant lab.

## Session extension

The hard cap keeps idle KASAN seats from lingering, but a real fuzzing session
runs longer, so research labs (only) are extendable:

- `Manager.Extend(id)` (`session.go`) pushes `Timing.MaxAge` out by `ExtendStep`
  (2h) up to `ExtendHardCap` (48h from `CreatedAt`), gated on `Module.IsResearch()`
  and a running session. It mutates `MaxAge` directly rather than via `Timing.Clamp`
  (which would re-cap to 8h); the reaper and `MarshalJSON` both read `ExpiresAt()`
  live off it, so nothing else needs updating and there is no persistence to write.
- Two entry points, both bounded by the same `Extend`: the researcher's own
  `POST /api/v1/sessions/{id}/extend` (auth + ownership), and the AI's grant-gated
  `POST /api/v1/ai/sessions/{id}/extend`. The AI is told to extend only when it is
  genuinely mid-task (see `aiExtendLine`), and otherwise to let the session lapse;
  `/ai/context` reports `expiresInSeconds` and warns ~5 min out.
- The AI grant slides with the session: `Extend` calls `extendGrant` (in-place bump
  of the grant store's expiry, no re-mint), and `aiGrantCreate` issues the research
  JWT out to the 48h ceiling so the researcher's pasted token keeps working across
  extends. The grant store expiry (and revocation) still bounds real access.

## What keeps a session alive

A session dies to whichever of its two clocks fires first, and in practice the
**idle** clock is the one that kills long research sessions - the 4h cap is rarely
reached. `Extend` is the only thing that moves the hard cap; everything below only
slides the idle deadline (`LastActivity + Timing.Idle`):

- **Terminal input** - the student typing (`terminal.go`), and the AI writing to
  the shared shell's stdin.
- **`AIExec`** (`aihub.go`) - marks activity as it writes the command. Note this
  fires **once, at launch**: `AIExec` returns within seconds while the job keeps
  running, so starting a long command does not by itself hold the session open.
- **Shell output** - `termHub.broadcast`, throttled to `outputActivityThrottle`
  (30s). This is what actually carries long jobs: a fuzz run or kernel build that
  prints anything keeps marking itself alive without the AI or the browser doing
  a thing. Added because the input-only signals above let a 90-minute build idle
  the session out from under itself at 60m.
- **`GET /ai/context`** - an AI poll counts as engagement, so a talky-but-active
  assisted session is not reaped between commands.
- **`POST /sessions/{id}/keepalive`** - the browser's presence ping, sent every
  60s while the lab tab is *visible*. No interaction required: a tab parked open
  on a second monitor while a job runs is the normal research pattern, and gating
  this on recent mouse/key activity reaped exactly those sessions.

`/ai/context` reports the clocks unmerged - `idleWindowSeconds`,
`hardCapExpiresInSeconds`, and `expiresInSeconds` (the soonest of the two, plus the
grant) - because the merged value is ambiguous precisely when it matters: each poll
resets idle, so `expiresInSeconds` reads ~`idleWindowSeconds` at every poll while
the cap is far off, and the AI could not tell "60m of idle window my silent job
will not refresh" from "60m of session left". A silent job longer than the idle
window still needs an `/extend` or a periodic poll; `aiExtendLine` says so.

## Infra / GitOps changes (labs node, AX41)

By hand on the AX41, mirroring the eBPF bring-up:
1. Build both kernels with Kata's `build-kernel.sh` + the combined config + each
   mitigation fragment; install the two `vmlinux` under
   `/opt/kata/share/kata-containers/`.
2. Add `configuration-fc-research-dev.toml` / `-hard.toml` (copies of
   `configuration-fc.toml` with `kernel =` pointed at each build) and register two
   containerd runtime handlers `kata-fc-research-dev` / `-hard`; restart containerd once.
3. `infra/runtimeclass/kata-fc-research-dev.yaml` + `-hard.yaml` (copies of
   `kata-fc-ebpf.yaml`; no `spec.overhead`, same reasoning as the others).
4. Add both handler names to `allowedRuntimeClasses` in `apps/labs/values.yaml` - the
   ValidatingAdmissionPolicy is fail-closed, so a runner pod with an unlisted class is
   denied at admission until this lands.

No new node. Two extra kernels are disk + two containerd handlers on the box already
in the cluster.

## Local dev

The lab *code* is developed locally under runc/k3d (the Tiltfile path), same as every
other lab: backend, registry, `Module` wiring, frontend, session-runner orchestration.
The dev machine is arm64 (Apple Silicon); the research kernel is x86 - so x86 images are
built with `buildx`, but the plumbing is arch-independent.

The kernel *behavior* (booting KASAN, running the exploit) is **prod-only**, because
local runc shares the host kernel and cannot load a custom one. This is not new and not
arch-specific: `kata-fc-ebpf` has the identical limitation. Custom kernels are exercised
on the AX41, which is x86 - the right arch for the research kernel, so there is no
mismatch there. The arm dev Mac never needed to run the kernel; it writes the code that
ships to the x86 node.

## Security review

This lab **intentionally defeats Wall 1** (the guest-kernel boundary) - popping the
guest kernel is the goal. Tenant isolation therefore rests entirely on:
- **Firecracker's VMM/jailer boundary** - which is exactly what Firecracker is built to
  provide (`ebpf-per-session.md`: "Firecracker's escape boundary is the jailer + minimal
  device model"; blast radius stays the throwaway VM);
- **Wall 2 NetworkPolicy** - default-deny, `NeedsEgress: false`, unchanged;
- **grant-gating to special users** - the exploit-a-kernel capability is never in the
  public catalog.

The reasoning is identical in shape to the eBPF security review and consistent with the
proven H1 escape-containment gate. Sign-off item: confirm the AX41
Firecracker/jailer posture is acceptable as the sole tenant boundary for a lab whose
purpose is guest-kernel compromise. Because it is grant-only to vetted users, the risk
surface is a known, small set.

## Capacity & cost

- **No new Hetzner purchase.** Both kernels live on the AX41.
- Research sessions are the heaviest tier (KASAN + eBPF, ~2 GB+). They are **grant-only**
  and expected to be rare, so they fit inside the existing `MemoryBudget=56Gi`; a running
  research session consumes existing seats (opportunity cost on the node), never new spend.
- If concurrency on the AX becomes the bottleneck, the lean-hardened variant (drop
  kprobes/Tetragon from `hard`, keep BTF) reclaims the ~880 MiB and is arguably more
  realistic - production kernels often lock eBPF down. `dev` keeps the full Defender.

## Arch scope

x86_64-only covers roughly 90% of the research value for cloud/server Linux driver
work, because the bug, the SLUB allocator, the reclaim objects, grooming, and the
data-only finish (`cred` overwrite, `modprobe_path`) are all architecture-agnostic.
Empirically (probed 2026-09-01): the GCP fleet is x86_64 (`6.8.0-gcp`, Ubuntu 22.04,
`virtio`); the one AWS box is arm64 Graviton (`7.0.0-aws`, Ubuntu 24.04, `ena`).

What x86-only genuinely cannot do, and would require paid ARM metal:
- **Android/mobile driver exploitation** (Qualcomm/Mali - the richest real driver surface);
- **arm64 hardware mitigations** - PAC, BTI, MTE (do not exist on x86);
- Graviton-specific cloud bugs.

Treat arm64 as a target-driven scope decision, not a completeness gap. Kernel *versions*
stay swappable config on the same node (add `6.8` to match the GCP fleet, or `7.0`) at
zero new cost.

## Phased rollout

- **P0 - code (local, runc): DONE + verified locally.** The `Kernel` selector, the
  two `Module` entries, the research track, the flag-capstone skip, the AI-grant TTL,
  the `HasDefender` tab wiring, the Topology-hide + About-panel UX, AND the real
  research workstation image (toolchain + driver + fuzzer + README). Exercised
  end-to-end under runc against `kata-fc`: session provisions, workstation carries the
  caps + toolchain + `/app`, the fuzzer compiles and runs, the Defender pane streams
  `commit_creds`. The CI airgap bake step for `research.tar` is wired
  (`.github/workflows/ci.yml`).
- **P1 - kernels (AX41): DONE.** `infra/cluster/kata-fc-research-node.sh` built + installed
  both `vmlinux-6.18.35-research-{dev,hard}`, wrote both TOMLs, and registered both
  containerd handlers on the AX41; the two RuntimeClasses are applied. Backend config +
  allowlist rode ArgoCD.
- **P2 - M0 acceptance: PASSED on both labs.** `sh /app/driver/load.sh`, `/app/fuzz/fuzz uaf`
  -> a clean KASAN `slab-use-after-free` report in `dmesg` (object + `kmalloc-64` cache),
  and the Defender tab streams `commit_creds`. Confirmed on `driver-research-dev` and on
  `driver-research-hard` (all four mitigations on; the report lands in a randomized cache -
  the harder groom). See the five prod-only fixes in the status block above. Driving the
  UAF the rest of the way to `uid 0` (grooming) is the researcher's exercise; the oracle
  and the primitives are all in place.
- **P3 - fast-follow:** `CONFIG_CFI_CLANG` on the hardened kernel (needs a clang build);
  full `syzkaller` in the image; a Module.symvers so `make` needs no
  `KBUILD_MODPOST_WARN`; lean-hardened variant if concurrency needs it.

## Open decisions for the owner

1. Kernel maintenance owner + cadence (same open question the eBPF kernel has).
2. ~~`dev` version pin~~ - **RESOLVED (2026-09-02): match Kata's shipped guest version +
   KASAN**, mirroring the `kata-fc-ebpf` build path (auto-detected by the node script),
   rather than a separate 6.12/6.8 pin. Kernel version stays swappable config at no new
   cost if a fleet-matching build is wanted later.
3. Full-eBPF-in-both vs lean-hardened (Defender everywhere vs more seats + more realism).
   Currently full-eBPF-in-both (Defender lit on both labs).
