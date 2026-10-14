# Nested-lab rewrite plan

**Goal:** each session attacks a *disposable nested k3s cluster* running inside its
own microVM (Kata/Firecracker) on the AX41 labs node. Winning a lab compromises a
throwaway cluster, never prod.

- **microVM = kernel wall** - contains pod escape.
- **nested k3s = API/namespace wall** - contains namespace/RBAC escape.

Fits AX41 at v1 scale (≤5 parallel, ~1.5 GB/session). Cost is software, not infra
(~same €85/mo). This is a **prerequisite for the first pod-escape / namespace-escape
modules**, but **infra-independent** - Path B bring-up (CX33 + join AX41) runs in
parallel and does not block it.

## Local ↔ prod fidelity
Same nested-k3s-in-pod unit both places. The only seam is the RuntimeClass:
- local (k3d): runner pod runs privileged, `runtimeClassName` empty → runc.
- prod (AX41): `runtimeClassName: kata-fc` → microVM.

## Phases & tasks

| Task | Phase | What |
|------|-------|------|
| A1 | runtime | Build the session-runner k3s image |
| A2 | runtime | Prove nested k3s in a pod on k3d |
| A3 | runtime | Extend the kata-fc RuntimeClass seam to the runner pod |
| B1 | provisioning | Rewrite `Create` - provision microVM + fetch nested kubeconfig (via Secret) |
| B2 | provisioning | Nested client-go client (dial the nested API) |
| B3 | provisioning | Apply module manifests into the nested cluster |
| B4 | provisioning | Rewrite `Delete` + reaper (tear down the microVM) |
| B5 | provisioning | Rewrite `Reconcile` (adopt/clean orphaned runners) |
| C1 | module format | Module → manifest bundle + entry-pod ref |
| C2 | module format | Author the first namespace-escape module |
| D1 | terminal | Bridge ttyd out of the microVM; rewrite proxy/TerminalTarget |
| D2 | terminal | Verify terminal into the nested cluster |
| E1 | cleanup | Runner-pod RBAC; delete the per-session NetworkPolicy path |
| F1 | verify | Containment proof (ns escape can't reach host / outer cluster) |
| F2 | verify | Tests for nested provisioning, teardown, reconcile |

## Critical path
`A1 → A2 → A3 → B1 → B3 → D1 → D2 → F1`
`C1 → C2` runs alongside (B3 and F1 depend on it). `B4/B5/E1/F2` hang off B.
Rough size: ~1-1.5 weeks focused; B is heaviest.

## Decisions (baked in)
1. **Local nesting = k3s-in-privileged-pod** (mirrors prod k3s-in-microVM), not vcluster.
2. **Replace, not coexist** - nested subsumes the pod-A/B + netpol path; retire it (`E1`). Reversible.
3. **Nested kubeconfig handoff** - runner writes it to an emptyDir; the backend
   reads it via `exec` (runner needs zero outer RBAC). Revised from "publish a
   Secret" after A2 - see requirement 4 below.

## A2 result - PROVEN (nested k3s runs in a pod on k3d, runc)

All three gates passed: nested node `Ready`, a pod scheduled inside the nested
cluster, and an authenticated call to the nested API from a separate outer pod
using the published kubeconfig. The model is sound. Getting there surfaced four
requirements the Go provisioner (`B1`) must encode - none optional:

1. **Private cgroup namespace.** Under runc the runner shares the host cgroup
   namespace, so the inner kubelet fights the outer cluster's `kubepods` and
   containerd dies. k8s exposes no field for this and the k3s image has no
   `unshare -C`, so the image ships a static `cgns-exec` helper that unshares
   `CLONE_NEWCGROUP` + remounts cgroup2 before k3s. No-op under Kata (microVM
   already isolates cgroups) via `SESSION_RUNNER_SKIP_CGNS=1`.
2. **Distinct nested CIDRs.** The nested k3s must not reuse the outer
   pod/service CIDRs (k3d default 10.42/10.43), or its kube-proxy hijacks the
   outer CoreDNS ClusterIP (10.43.0.10) and breaks the runner's own DNS →
   cascades into "can't pull pause". Runner uses 10.52/10.53, cluster-dns
   10.53.0.10 (overridable via `SESSION_*_CIDR` env).
3. **Privileged pod in a non-`restricted` namespace.** The runner is privileged
   by necessity; `labs` enforces `restricted` and rejects it. Runner pods need
   their own namespace at `privileged` PSA. This does not weaken isolation:
   lab-content pods now live inside the *nested* cluster, governed by its PSA,
   so outer `restricted` no longer applies to them.
4. **Kubeconfig handoff = backend exec-cats `/output/kubeconfig`** (decision 3).

Reproduce with `services/session-runner/poc-pod.yaml` (see its header).

## A3 result - PROVEN (nested k3s in a Firecracker microVM on the AX41)

All three gates passed on the real Hetzner bare-metal box (Ubuntu 24.04, kernel
6.8) under `kata-fc` (Firecracker): nested node `Ready`, a pod scheduled inside
the nested cluster, and the nested API reachable from outside the microVM with a
valid pod-IP cert. Verified with the baked image, no command overrides.

Big architectural finding: **Firecracker + devmapper is required, not optional.**
Kata's virtio-fs VMMs (clh/qemu) get the nested *control plane* up but the
nested **kubelet** fails - it needs a real block-backed rootfs (`/var/lib/kubelet`
device lookup fails on virtio-fs). Firecracker uses a block rootfs via the
containerd `devmapper` snapshotter, which satisfies it. So the original prod
choice (kata-fc) is correct for a deeper reason than expected.

Requirements this pins down:

**Runner pod spec under Kata (B1 must emit):**
- `runtimeClassName: kata-fc`.
- **Not** `privileged` - a privileged pod makes Kata try to hotplug host block
  devices (incl. the devmapper loop devices) into the VM and fails. Use
  `capabilities.add: [SYS_ADMIN, NET_ADMIN, NET_RAW, MKNOD, SYS_RESOURCE,
  SYS_PTRACE, SYS_CHROOT]` instead. (Local runc/k3d still uses `privileged` per
  A2 - this is the one spec difference the backend keys off `RuntimeClass`.)
- cgns-exec still runs (writable cgroup2 needed under Kata too).

**Baked into the image (entrypoint, work under both runc and Kata):**
- remount `/proc/sys` rw (kubelet writes kernel.panic etc; it's in runc's
  readonlyPaths).
- create `/dev/kmsg` (absent in Kata's minimal guest; kubelet opens it).
- `--snapshotter=native` (nested containerd can't overlay-mount on either the
  outer overlay or Kata's rootfs).
- distinct nested CIDRs 10.52/10.53 (from A2).

**Host prerequisites on the labs node (prod runbook - kata-fc + devmapper):**
- kata-static 3.32 + Firecracker to `/opt/kata`; `containerd-shim-kata-v2` on PATH.
- containerd `devmapper` snapshotter backed by a thinpool (test used a loopback
  pool - prod needs a real LV/disk-backed pool with a boot-time recreate).
- containerd drop-in registering the `kata-fc` runtime (`snapshotter =
  "devmapper"`) + `RuntimeClass/kata-fc`.
- `privileged_without_host_devices` does NOT save a privileged pod here - hence
  caps, not privileged.

## Status
Phase A ✓ (A1 image, A2 runc/k3d, A3 Firecracker/AX41 - runner image
self-sufficient in both).

Phase B ✓ **code-complete** (compiles, unit-tested, built on A2/A3-proven
runtime primitives):
- B1 `createNested` provisions the runner pod (`runner.go` - runc-privileged vs
  Kata-caps seam) and orchestrates the flow.
- B2 fetch nested kubeconfig via pod exec + `nestedRESTConfig` (`nested.go`).
- B3 `applyNestedManifests` (dynamic client + discovery RESTMapper).
- B4 `Delete`/reaper branch on `Session.Nested` → `deleteRunner` (one pod delete
  collapses the whole nested cluster).
- B5 `Reconcile` second pass re-adopts `app=session-runner` pods.
Config wired: `LABS_RUNNER_NAMESPACE` / `LABS_RUNNER_IMAGE` (chart + env).

Legacy pod path kept for now (coexists) - nested provisioning is gated on
`module.Nested != nil`. This is the temporary-coexistence form of decision 2;
`E1` retires the legacy path once a nested module (`C2`) replaces the last user
of it.

**Verification boundary:** unit tests cover the pure builders (runc-vs-Kata
seam, labels, tolerations, downward API). Full backend-in-cluster e2e (the
backend actually driving `createNested`) needs a nested module to provision
(`C2`), the `labs-runners` namespace + backend exec/pod RBAC (`E1`), and a
redeploy - that lands as `D2`/`F1`. The underlying operations (runner boots,
kubeconfig publishes, nested API reachable in-cluster, pod schedules inside
nested) are already proven by A2/A3.

Next: `C1` is folded in (Module gained `Nested *NestedSpec`). Remaining: C2
(author a nested module), D1/D2 (terminal into nested), E1 (RBAC + retire
legacy), F1/F2 (containment + integration tests).

## LOCAL E2E - PROVEN (2026-07-16)

`C2` + `E1` (runner namespace + RBAC) landed and the whole flow ran end to end
on k3d against the real backend (rebuilt, redeployed):

1. `POST /sessions {module: ns-escape}` → backend created `runner-<id>` in
   `labs-runners`, booted its nested k3s, fetched the kubeconfig by exec,
   applied the module bundle → session `running`.
2. **Containment held:** the `dev`/`production` namespaces, `flag` secret, and
   `analyst-secret-reader` ClusterRoleBinding exist ONLY inside the nested
   cluster - the outer cluster has none of them.
3. **Exploitable:** from the `foothold` pod, the `analyst` SA `can-i get secrets
   -n production` → yes, and read the flag
   (`k8labs{cross_ns_secret_read_via_overbroad_clusterrolebinding}`).
4. `DELETE /sessions/<id>` → 204, runner pod gone, whole nested cluster with it,
   session 404.

So B (provisioning), C2 (module), and E1 (RBAC/namespace) are verified together
in-cluster, not just unit-tested. Note: the exploit was driven via `kubectl
exec` manually - the platform **terminal into the nested cluster is still D1/D2**
(a UI user has no shell yet). `E1`'s "remove per-session NetworkPolicy code" half
is deliberately deferred until the legacy suid module is migrated off the pod
path; the runner-RBAC + namespace half is done.

Deploy note: local backend deploy hit a sealed Vault (restart 28h earlier) -
unsealed from `secrets/vault-unseal-keys.json` per the vault runbook. Not related
to the rewrite.

## D (terminal) - DONE, exec-based

Chose exec over the plan's original "bridge ttyd out" after finding the frontend
uses an iframe→ttyd and that nested clusters can only pull public images. The
exec approach: the backend execs `/bin/sh` into the foothold pod via the nested
kubeconfig it already holds and bridges it to one browser websocket; the
frontend renders its own xterm.js. Fewer prod hops (no kube-apiserver pod-proxy
in the path), keepalive pings for Cloudflare's ~100s idle cut, and no per-module
ttyd image. See the comparison in this session's history.

- **D1** - backend `nestedTerminal` (exec bridge + PTY resize + ping keepalive,
  `internal/labs/terminal.go`), `TerminalProxy` branches on `Session.Nested`;
  frontend `NestedTerminal.tsx` (xterm), `LabPage` renders it for nested
  modules. Protocol: binary=stdin, text `{cols,rows}`=resize, binary=stdout.
  Auth via the same-origin cookie (a browser ws can't send a bearer header).
  Also added `waitForNestedPodReady` so a session isn't "running" until the
  foothold is up (else the first connect races its image pull).
- **D2** - verified two ways: a raw ws client drove the full backend path and
  read the flag through the shell; and in the real browser the xterm terminal
  renders, connects to the nested foothold (`/apps #` prompt), and streams typed
  input. (Synthetic Enter from browser automation doesn't reach xterm's hidden
  textarea - a harness quirk, not a bug; the ws-client test proved end-to-end
  execution.)

A UI user now gets a working shell into the nested cluster. Remaining: F1
(containment proof) and F2 (integration tests).

## Async provisioning (fix)

First real-UI use surfaced a bug: nested provisioning was synchronous, so the
`POST /sessions` request blocked ~1 min (boot k3s + pull the module images into
a fresh cluster) - past the backend's 90s write timeout. The response got cut,
the session errored, and `waitForNestedPodReady`'s timeout reaped the runner ->
the user saw a blank terminal.

Fixed by making nested provisioning asynchronous: `Create` registers the session
`starting` and returns immediately; `provisionNested` does the slow build in a
background goroutine (5-min budget, its own context) and flips the session to
`running`/`error`. The frontend (`LabPage`) polls `GET /sessions/{id}` and shows
a "Provisioning your isolated cluster…" spinner until `running`, then renders the
terminal. Verified in-browser: Start -> spinner -> live shell, no timeout.

## Image-pull optimization (done)

The per-session foothold pull (~1 min) is gone. Two changes:
1. Replaced the bloated `alpine/k8s` (~850 MB) foothold with a tiny purpose-built
   one - `services/session-runner/foothold` = alpine + kubectl + curl/jq/bash
   (~24 MB, built locally, never on a public registry).
2. Bake it into the session-runner: the runner's build `docker save`s the
   foothold into `airgap/foothold.tar` (gitignored) and `COPY`s it to
   `/opt/airgap/`; the entrypoint drops it into k3s's `agent/images/` before
   boot, so every nested cluster imports it locally. The module references it
   with `imagePullPolicy: Never`.

Result verified in-cluster: foothold runs from the preloaded image (no pull),
kubectl works (flag read), and session-to-`running` dropped ~60s → ~16s (the
remainder is nested k3s boot, not image pull). Adding another module image to
the fast path = add it to the airgap bake the same way. Build note: CI's
session-runner build must run the same two-step (foothold build + save) before
the runner build; arch follows `TARGETARCH` (arm64 local, amd64 prod).

## F1 - containment (network wall done; kernel wall = prod)

Adversarial test found a real hole and fixed it. With no policy on `labs-runners`,
a nested-cluster attacker could reach the outer kube-apiserver (10.43.0.1) and the
Backend over the pod network - nested traffic SNATs through the runner pod, so it
rides the runner pod's (previously unrestricted) egress. The microVM does NOT stop
this (L3 routing, not a kernel boundary), so it was a prod hole too.

Fix: `apps/labs/templates/networkpolicy-runners.yaml` - default-deny on
`app=session-runner` + allow DNS + allow internet (nested image pulls) EXCEPT the
cluster pod/service CIDRs + ingress only from the Backend on 6443. Verified after
applying: outer API + Backend `blocked`, internet still reachable, the lab exploit
still works, other sessions' runners unreachable (10.42/16 blocked), and a fresh
session still provisions in ~14s.

Remaining F1 half - the kernel/host wall (can a pod escape reach the host?) - is
only meaningful under Kata/Firecracker, i.e. on the AX41; it rides the prod
bring-up (A3 proved the microVM boots, this proves it contains). Local runc has no
host wall by design.

## F2 - tests (done)

Fast fake-clientset tests over the orchestration branches a refactor is likeliest
to break (`internal/labs/nested_lifecycle_test.go`): Reconcile re-adopts both
nested (`app=session-runner`) and legacy (`app=lab`) sessions with the right
`Nested` flag; Delete routes nested→runner-pod vs legacy→pod/service; Create
routes a nested module to the nested path. Required a small testability refactor:
`Manager.clientset` is now `kubernetes.Interface` (main.go passes a proper nil
interface when there's no cluster). Full labs suite green under `-race`. The
heavy provisioning internals (exec, dialing the nested API) aren't fakeable and
stay covered by the manual k3d + AX41 e2e.

## Plan complete - 15/15

A ✓ (runtime) · B ✓ (provisioning) · C1/C2 ✓ (module format + first ns-escape
module) · D ✓ (exec terminal) · E1 ✓ (runner RBAC/ns) · F1 ✓ (network-wall
containment - found + fixed a real hole) · F2 ✓ (tests). Plus async provisioning,
the provisioning-spinner UX, and the baked-image fast start (~16s).

Carrying into prod bring-up (Path B): (a) F1's kernel/host wall - only meaningful
under Kata/Firecracker, rides the AX41; (b) legacy pod-path retirement once suid
migrates; (c) CI two-step runner build. Prod gap at plan time: CX33 control plane not yet created (built since).

## Path to private beta (approved 2026-07-17)

> **Status (2026-08): SHIPPED.** The G/H/I gates below are complete - prod is
> live as a 2-node kubeadm cluster (Path B). Kept as the
> executed plan of record.

Bar: private beta, few trusted users, fully demonstrable on the SAME tech.
Isolation + workability must be FULLY ready; more labs / per-user security
layers / abuse hardening are explicit fast-follows (additive, same stack).

Phase G - prod bring-up (infra):
  G1 Terraform CX33 control plane (infra/cloud - only thing not yet created).
  G2 kubeadm init on CX33 + Cilium + WireGuard CNI.
  G3 installimage + kubeadm join the AX41 (203.0.113.10) as labs worker.
  G4 kata-deploy under kubeadm containerd + devmapper snapshotter (A3 req).
  G5 bootstrap.sh: DNS, OAuth, GHCR pull secret, Vault unseal.
  G6 Deploy app (backend + labs Helm) via ArgoCD.
  (fold in: CI two-step runner build before G6 uses the GHCR image.)

Phase H - isolation gate (go/no-go; fix the model before building on it if red):
  H1 Kernel-wall proof on AX41 under Firecracker: pod escape stays in microVM.
     Non-negotiable, prod-only (local runc has no host wall).
  H2 Network-wall re-verify in prod: runner NetworkPolicy blocks outer-API /
     backend under Cilium (F1 was local/k3d).

Phase I - workability shakeout:
  I1 ns-escape on prod: provision->terminal->exploit->reap, repeated, no wedge.
  I2 Reconcile + reaper survive a backend restart in prod.
  I3 2-3 concurrent sessions coexist.

Green I-phase = isolation-ready, demonstrable private beta.

Fast-follow (post-beta): egress caps, capacity/admission limits, more labs,
per-user security layers, ops/alerting, legacy pod-path retirement.

Critical path: CX33 -> join AX41 -> kata/devmapper -> deploy -> kernel wall -> shakeout.
