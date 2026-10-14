---
kind: concept
ey: CONCEPT 2
eyt: Sharing a pod
tag: foundation
title: Containers in one pod share the pod's identity
aha: A pod is a shared apartment. Add a second tenant and they get their own copy of the house keys - the same keys the first tenant uses.
diagram:
  caption: two containers, one mounted token
  shapes:
    - {type: frame, x: 45, y: 100, w: 250, h: 110, label: 'one pod (app + sidecar)', delay: 0}
    - {type: box, x: 70, y: 150, w: 90, h: 44, label: 'app', delay: 0.4}
    - {type: box, x: 180, y: 150, w: 90, h: 44, label: 'sidecar', tone: danger, delay: 0.6}
    - {type: wire, from: {x: 295, y: 150}, to: {x: 400, y: 150}, tone: danger, delay: 0.9}
    - {type: box, x: 400, y: 118, w: 165, h: 66, label: 'token', sub: 'shared by both', tone: accent, delay: 1.1}
---
A pod can hold more than one `container`. An extra one running alongside the main
app is called a `sidecar` - service meshes and log shippers work this way. The
containers in a pod share resources, and crucially they share the pod's identity:
the `ServiceAccount` `token` is mounted into the pod, reachable by every container
in it.

So a sidecar you add to a pod is not a guest with limited access. It wakes up
holding the same `token` the app holds, and can make the same API calls the app
can. Put your container in the payments pod, and it is payments.
