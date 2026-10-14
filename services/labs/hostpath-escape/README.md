# hostpath-escape - reading the node via a hostPath mount

A container-escape lab in the nested-cluster shape. The student's foothold pod
was given a `hostPath` mount of the node's root filesystem; the flag is a
root-owned file on the node, reachable only through that mount.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/hostpath-escape.yaml`), applied into the
session's disposable nested k3s cluster. The foothold uses the shared
`k8labs-foothold` image; an initContainer plants the flag on the node through the
same host mount. No per-module image. The flag is bait (see `_template`).

## Containment

"The node" is the disposable nested cluster's node - the session-runner's **Kata
microVM guest** in prod, the runner pod's own container under **runc** locally.
Escaping it compromises only the throwaway session. This is why the lab is safe
to hand out: the real boundary is the microVM, not the container the student
breaks.

## The flaw

The `workstation` pod declares a `hostPath` volume of `/` mounted at `/host`
(read-only for the shell container; an initContainer uses a writable mount to
plant the flag). A hostPath of the node root dissolves container isolation - the
pod reads the node's entire filesystem with an ordinary file read. RBAC is not
involved, because nothing becomes an API call.

## Solution (spoiler)

```sh
mount | grep /host          # or: ls /host  -> the node's root filesystem
cat /host/opt/k8labs/flag.txt
# k8labs{hostpath_root_mount_reads_the_whole_node}
```

## The fix

Enforce PodSecurity `baseline`/`restricted` (or a policy engine) on namespaces
running general workloads - both forbid `hostPath`. A container's isolation is
only as strong as the pod spec it was admitted with; the dangerous fields
(`hostPath`, `privileged`, `hostPID`, `hostNetwork`) are admission concerns, not
RBAC ones.

## What a defender sees

There is no API event for the flag read - it is a filesystem access on the node.
The observable signal is at admission (a pod requesting a sensitive `hostPath`)
and, with in-guest telemetry, a process in the pod reading node-owned paths. Host
eBPF under Kata cannot see inside the nested cluster; in-cluster telemetry is the
way to observe it, the same design note as `ns-escape`.

## Verification status

Exploit is correct-by-construction (standard hostPath read) and the manifest is
valid. The full exploit-run happens inside the session's isolated nested node;
it was not run on the shared dev cluster (escaping there would land on the real
k3d node) - see the container-escape note in the lab-authoring guide.
