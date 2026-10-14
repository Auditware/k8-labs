---
kind: concept
ey: CONCEPT 2
eyt: Your identity
tag: foundation
title: Your pod already is an account - its token is mounted
aha: There is no password to find. The badge is clipped on, and anyone who reaches the shell is wearing it.
diagram:
  caption: the pod presents its token on every call
  shapes:
    - {type: box, x: 50, y: 115, w: 170, h: 70, label: 'pod', sub: 'your shell', delay: 0}
    - {type: label, x: 135, y: 205, text: 'mounted token file', delay: 0.3}
    - {type: wire, from: {x: 220, y: 150}, to: {x: 300, y: 150}, delay: 0.5}
    - {type: box, x: 300, y: 115, w: 160, h: 70, label: 'ServiceAccount', sub: 'sa-imper', tone: accent, delay: 0.7}
    - {type: wire, from: {x: 460, y: 150}, to: {x: 530, y: 150}, tone: accent, delay: 0.9}
    - {type: box, x: 530, y: 115, w: 70, h: 70, label: 'API', delay: 1.1}
---
Your pod runs as a `ServiceAccount` - a cluster-recognised identity named
`sa-imper`. Kubernetes mounts that account's `token` inside the pod at a fixed
path, `$ ls /var/run/secrets/kubernetes.io/serviceaccount/`, and `kubectl` uses
it automatically.

So you are not trying to become someone - you already are `sa-imper`. The
question is only what `sa-imper` was allowed to do, and the answer, on its own,
is almost nothing.
