---
kind: concept
ey: CONCEPT 2
eyt: Pod identity
tag: foundation
title: Every pod can carry an identity of its own
aha: It is a badge clipped to the pod, not to you. Anything running in that pod wears the badge - including code that got there in ways nobody intended.
diagram:
  caption: the pod presents its badge on every API call
  shapes:
    - {type: box, x: 50, y: 115, w: 170, h: 70, label: 'pod', sub: 'your shell', delay: 0}
    - {type: label, x: 135, y: 205, text: 'mounted token file', delay: 0.3}
    - {type: wire, from: {x: 220, y: 150}, to: {x: 300, y: 150}, delay: 0.5}
    - {type: box, x: 300, y: 115, w: 150, h: 70, label: 'ServiceAccount', sub: 'analyst', tone: accent, delay: 0.7}
    - {type: wire, from: {x: 450, y: 150}, to: {x: 520, y: 150}, tone: accent, delay: 0.9}
    - {type: box, x: 520, y: 115, w: 80, h: 70, label: 'API', delay: 1.1}
---
A pod can run as a `ServiceAccount` - an identity the cluster recognises. When it
does, Kubernetes mounts a `token` inside the pod, at a fixed path on disk -
`$ ls -l /var/run/secrets/kubernetes.io/serviceaccount/` lists it.

Any process in that pod can read the token and use it to talk to the cluster's
API. That is the point: it is how a pod asks the cluster to do things. It also
means that if someone gets a shell in the pod, they inherit the identity
immediately, without stealing a password.
