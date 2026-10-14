---
kind: concept
ey: CONCEPT 1
eyt: A pod's identity
tag: foundation
title: A pod carries its identity as a file on disk
aha: The credential is not in a vault the pod unlocks. It is a plain file, sitting in the filesystem, readable by anything that reaches the shell.
diagram:
  caption: the token is just a file inside the pod
  shapes:
    - {type: box, x: 60, y: 105, w: 210, h: 96, label: 'pod', sub: '/var/run/.../token', delay: 0}
    - {type: wire, from: {x: 270, y: 153}, to: {x: 380, y: 153}, tone: accent, delay: 0.5}
    - {type: box, x: 380, y: 120, w: 190, h: 66, label: 'ServiceAccount', sub: 'what the token proves', tone: accent, delay: 0.8}
---
A pod usually runs as a `ServiceAccount` - a cluster identity. When it does,
Kubernetes mounts that account's `token` inside the pod, as a file at
`/var/run/secrets/kubernetes.io/serviceaccount/token`. Any process in the pod
can read it and use it to authenticate to the API.

This is ordinary and necessary - it is how a pod talks to the cluster. But it
means a pod's power is not locked behind a password. It is a file, and whoever
is standing in that pod's filesystem can pick it up.
