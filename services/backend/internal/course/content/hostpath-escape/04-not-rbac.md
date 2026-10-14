---
kind: concept
ey: CONCEPT 3
eyt: Why RBAC cannot help
tag: the machinery
title: This is filesystem access, so RBAC never gets a say
aha: RBAC guards the front door - the API. A hostPath mount walks in through the floor, where there is no guard to check a badge.
diagram:
  caption: the read never touches the API
  shapes:
    - {type: box, x: 50, y: 116, w: 165, h: 66, label: 'you', sub: 'cat /host/...', delay: 0}
    - {type: wire, from: {x: 215, y: 149}, to: {x: 320, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 320, y: 116, w: 150, h: 66, label: 'node disk', tone: danger, delay: 0.7}
    - {type: box, x: 320, y: 55, w: 150, h: 44, label: 'API + RBAC', sub: 'not involved', delay: 1.0}
---
Every earlier lab turned on a permission - an RBAC verb the account did or did not
hold. This one does not. Reading a file through a `hostPath` mount is an ordinary
filesystem read on the `node`. It never becomes an API request, so there is no
`kubectl auth can-i` to consult and no Role to tighten.

That is what makes host mounts so dangerous. All the careful RBAC in the cluster
governs the API, and this simply goes around it. The only thing that can stop a
`hostPath` of `/` is refusing to let the pod have it in the first place - which is
what admission controls like `PodSecurity`, at its `baseline` or `restricted`
level, exist to do.
