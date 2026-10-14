# session-runner

The container that **is a disposable Kubernetes cluster**. Every namespace /
RBAC / cluster-takeover module runs its attack against one of these, one per
session. When a student wins, they own a throwaway k3s that gets destroyed at
logout - not prod, not another tenant.

This image ships **no vulnerability**. It is generic, reused infrastructure -
the arena, not the challenge. Lab content is applied *into* it at provision
time (see `C1`/`C2` for module format, `B3` for the apply step).

## Two walls (why this exists)

- **microVM (Kata/Firecracker) around the pod** = kernel wall - contains a pod
  escape (the "host" the student breaks out to is a throwaway guest kernel).
- **the nested k3s inside it** = API wall - contains a k8s escape (the cluster
  the student becomes admin of is disposable).

A microVM alone can't contain a k8s-API attack, because a real API server would
live *outside* the VM, reachable over the network. Putting the target API
server *inside* the same VM is the whole point. See
[docs/architecture/nested-lab-rewrite.md](../../docs/architecture/nested-lab-rewrite.md).

## What's inside

Built `FROM rancher/k3s` - the same k3s + containerd + CNI bundle that k3d runs
to give you a cluster in a container. We only replace the entrypoint
(`entrypoint.sh`):

1. boot single-node, SQLite-backed k3s with lab-irrelevant add-ons disabled
   (traefik, servicelb, metrics-server, helm-controller, cloud-controller);
2. wait for k3s to write its kubeconfig;
3. publish it to `/output/kubeconfig`, rewriting the server address from
   `127.0.0.1:6443` to the pod IP (`SESSION_API_HOST`, injected via the
   downward API) so the backend can dial it.

k3s stays PID 1 for correct signal handling / process reaping.

## The local ↔ prod seam

Identical image and pod shape both places. The **only** difference is the pod's
`runtimeClassName`, set by the backend, never here:

| | runtimeClassName | isolation |
|---|---|---|
| local (k3d, Mac - no KVM) | empty → runc | privileged pod, no microVM |
| prod (AX41) | `kata-fc` → Firecracker | microVM guest kernel |

runc locally is a hardware floor, not a choice: Firecracker needs KVM, which
Docker Desktop on macOS can't expose. A3 is the gate that covers the Kata-only
risks (chiefly: does the stripped guest kernel carry the modules inner k3s
needs).

## Env knobs

| var | default | purpose |
|---|---|---|
| `SESSION_API_HOST` | `127.0.0.1` | address written into the published kubeconfig; set to the pod IP |
| `SESSION_API_PORT` | `6443` | matching port |
| `OUTPUT_KUBECONFIG` | `/output/kubeconfig` | where the rewritten kubeconfig lands |
| `K3S_EXTRA_ARGS` | (empty) | escape hatch, e.g. `--snapshotter=native` if overlay-on-overlay nesting fails |

## Proving it (A2)

`poc-pod.yaml` is the throwaway harness. See the header of that file for the
exact apply / wait / exec / dial sequence. Pass = the whole rewrite is sound.
