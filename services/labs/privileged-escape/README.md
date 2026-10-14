# privileged-escape - breaking out of a privileged, hostPID pod

A container-escape lab in the nested-cluster shape, and one of three
**attack+detect** labs (`Module.EBPF`, alongside `ebpf-detect` and
`node-token-harvest`). The student's foothold pod is `privileged` and shares the
host PID namespace (`hostPID`); the flag is a root-owned file on the node, reached
by breaking out via `/proc/1/root` - then the student turns to the Defender pane
and watches an in-guest eBPF sensor (Tetragon) catch the breakout.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/privileged-escape.yaml`), applied into
the session's disposable nested k3s cluster. The foothold uses the shared
`k8labs-foothold` image; a privileged initContainer plants the flag on the node
via the same `/proc/1/root` path. No per-module image. The flag is bait.

## Why /proc/1/root (not nsenter)

The classic breakout uses `nsenter`, but the foothold image ships no util-linux,
so `nsenter` is unavailable. `/proc/1/root` needs only coreutils: `hostPID` makes
the node's init `PID 1` inside the pod, and `privileged` grants the capability to
follow another process's root, so `/proc/1/root` is the node's filesystem. This
is the intended, image-independent path.

## Containment

"The node" is the disposable nested cluster's node - the session-runner's Kata
microVM guest in prod, the runner pod's own container under runc locally.
Escaping it compromises only the throwaway session (the same containment as
suid-privesc).

## The flaw

The `workstation` pod sets `hostPID: true` and each container is
`securityContext.privileged: true`. Neither alone suffices - privileged without
hostPID cannot see PID 1; hostPID without privileged is denied the read - but
together `/proc/1/root` is readable.

## Solution (spoiler)

```sh
id                                        # root in the container
cat /proc/1/root/etc/hostname             # the node's name, not the pod's - escape confirmed
cat /proc/1/root/opt/k8labs/flag.txt
# k8labs{privileged_hostpid_proc_1_root_is_the_node}
```

## The fix

Enforce PodSecurity `baseline`/`restricted` (or a policy engine) - both forbid
`privileged` and `hostPID`. Reserve them, if ever, for a small set of audited
system components. Container isolation is a property of the admitted pod spec.

## What a defender sees

No API event - the read is on the node filesystem. Signals are at admission (a pod
requesting `privileged`/`hostPID`) and, with in-guest telemetry, a privileged
process dereferencing PID 1's root. This lab makes the second observable: a
Tetragon DaemonSet in the `tetragon` namespace runs a `TracingPolicy`
(`proc-root-access`) that hooks the `openat(2)` syscall and posts an event
whenever a process opens a path with prefix `/proc/1/root` - the act of reaching
through the node init's root, i.e. the breakout itself. The event stream is tailed
into the student's Defender pane (`app=tetragon`, container `export-stdout`). The
syscall hook is essential here: `/proc/1/root` is a magic symlink, so the LSM
`security_file_permission` hook would report the *resolved* node path
(`/opt/k8labs/flag.txt`) and never see the `/proc/1/root` framing - only the raw
syscall argument carries it. Host eBPF under Kata cannot see inside the nested
cluster, which is why the sensor runs *inside* the session (see
`docs/architecture/ebpf-per-session.md`).

## Verification status

**Verified end-to-end (2026-07-29, local runc/k3d).** A real nested session was
provisioned (session-runner boots k3s, `SESSION_EBPF=1` imports the Tetragon
image), the bundle applied, and the breakout run: Tetragon loaded the policy
cleanly (`Added TracingPolicy with success ... proc-root-access`), the escape
succeeded (`/proc/1/root/etc/hostname` returned the nested node name and the flag
read), and the Defender feed showed the matching `process_kprobe` -
`function_name __arm64_sys_openat`, `policy_name proc-root-access`, `string_arg`
`/proc/1/root/opt/k8labs/flag.txt`. The nested "node" the exploit reaches is the
disposable runner container (runc locally, Kata microVM in prod) - `/proc/1/root`
resolved to the runner, never the shared dev node - so it stays contained to the
throwaway session, exactly the boundary the lab-authoring guide describes.
