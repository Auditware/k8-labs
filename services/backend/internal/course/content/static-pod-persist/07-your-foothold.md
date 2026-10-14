---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: You cannot read the node, but you can write the one directory that runs anything. So you write yourself a helper with the access you lack.
revealLabel: how to start
diagram:
  caption: drop a static pod, collect the loot
  shapes:
    - {type: box, x: 45, y: 116, w: 165, h: 66, label: 'workstation', sub: 'writes /manifests', tone: danger, delay: 0}
    - {type: wire, from: {x: 210, y: 149}, to: {x: 300, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 300, y: 116, w: 165, h: 66, label: 'static pod', sub: 'mounts node /', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 465, y: 149}, to: {x: 525, y: 110}, tone: accent, delay: 0.95}
    - {type: box, x: 500, y: 60, w: 115, h: 50, label: 'flag', sub: 'loot back', tone: accent, delay: 1.15}
---
Your session drops you into a pod called `workstation` in the `lab` namespace.
Look at what you can write - `$ ls -la /manifests` is the kubelet's static-pod
directory. Your container cannot read the node otherwise, and the flag is a
root-owned file under the node's `/opt`.

Write a `static pod` manifest into `/manifests` (image `k8labs-foothold:1.31`,
`imagePullPolicy` set to `Never`) that mounts the node's `/`, reads the flag, and
copies it to a file back under the manifest directory. Wait a few seconds for the
kubelet to start it, then read your loot from `/manifests`. The `API server` would
never have let your account run that pod - the kubelet did not ask it.

**If nothing happens, the manifest is the suspect, not the kubelet.** The kubelet
silently ignores a file it cannot parse, so validate your YAML first
(`$ kubectl apply --dry-run=client -f your.yaml` catches the common mistakes).
And do not go looking for the pod by the name you gave it: the kubelet publishes a
read-only mirror named `<your-pod-name>-<node>` in the manifest's namespace
(`default` if you set none), so `$ kubectl get pod <name>` returns NotFound even
when it is running. You do not need that mirror anyway - your result comes back
through the file you wrote into `/manifests`, not through the API.
