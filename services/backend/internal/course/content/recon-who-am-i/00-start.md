---
kind: concept
ey: START
eyt: Who Am I
tag: what you'll learn
title: No kubectl. Real pods don't have it either.
aha: The tool you reach for is a luxury of your laptop. Inside a compromised workload you have a token and curl - and that is enough to run the cluster.
revealLabel: why this matters
diagram:
  caption: a token and curl reach the same API kubectl did
  shapes:
    - {type: box, x: 60, y: 116, w: 200, h: 64, label: 'curl + token', sub: 'no kubectl', delay: 0}
    - {type: wire, from: {x: 260, y: 148}, to: {x: 370, y: 148}, delay: 0.4}
    - {type: box, x: 370, y: 116, w: 210, h: 64, label: 'API server', sub: 'kubernetes.default', tone: accent, delay: 0.7}
---
Open a shell here and type `kubectl`. It is gone - and that is the point.

A real compromised workload is an application container: it has `curl`, maybe
`jq`, and a mounted `ServiceAccount` `token` - but almost never a configured
kubectl. Leaning on kubectl teaches a habit that breaks the moment you land
somewhere real. This whole lab is done with the `token` and `curl`, straight
against the API server, the way an attacker actually works.
