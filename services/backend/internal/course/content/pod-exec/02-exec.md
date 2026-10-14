---
kind: concept
ey: CONCEPT 2
eyt: The exec verb
tag: the machinery
title: exec is not "run a command" - it is "become this pod"
aha: Think of it less like sending a command and more like teleporting into the pod. Once you are inside, everything the pod has is within arm's reach.
diagram:
  caption: exec puts you inside, not beside
  shapes:
    - {type: box, x: 55, y: 116, w: 175, h: 66, label: 'you', sub: 'kubectl exec', delay: 0}
    - {type: wire, from: {x: 230, y: 149}, to: {x: 350, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 350, y: 105, w: 220, h: 90, label: 'inside the target pod', sub: 'its shell, its token', tone: danger, delay: 0.7}
---
Kubernetes lets an authorised caller open a shell inside a running pod - an
operation called `exec`, run with `$ kubectl exec`. It is the cluster's version
of SSH-ing into a container - you get an interactive session as whatever user
that container runs.

The permission is an RBAC verb on a subresource, `pods/exec`. And the crucial
part is where it lands you - not next to the pod, but *inside* it, sharing its
filesystem and, therefore, its mounted `token`. You did not copy the identity.
You walked into the room where it is kept.
