---
kind: concept
ey: CONCEPT 5
eyt: Do not wedge the cluster
chip:
  label: WATCH OUT
  tone: warn
title: Scope the webhook, or you break everything including yourself
aha: A webhook that intercepts all pods, and fails closed when its server hiccups, does not just backdoor the cluster - it stops it from starting any pods at all.
diagram:
  caption: narrow the target, fail open
  shapes:
    - {type: box, x: 55, y: 116, w: 185, h: 66, label: 'match everything', sub: 'fail closed', tone: danger, delay: 0}
    - {type: wire, from: {x: 240, y: 149}, to: {x: 350, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 350, y: 116, w: 205, h: 66, label: 'cluster wedged', sub: 'no pods start', tone: warn, delay: 0.7}
---
Power this broad is easy to misfire. Two settings keep it controlled. A
`namespaceSelector` limits the webhook to the namespaces you care about - here,
`production` - instead of every pod in the cluster, including the system pods that
keep it running. And `failurePolicy` decides what happens if your server is
unreachable: set it to ignore failures, so a hiccup does not block pod creation
cluster-wide.

Get these wrong - match everything, fail closed - and the first time your server
stutters, nothing can schedule, the node's own components included. Scope tightly
and fail open: quieter, more durable, and it does not draw attention by breaking
the cluster.
