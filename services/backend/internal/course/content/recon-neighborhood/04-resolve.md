---
kind: concept
ey: CONCEPT 3
eyt: Name to address
tag: technique
title: Resolve the name your config just handed you
aha: With the name in hand, the blocked API is irrelevant - you ask DNS instead, and DNS obliges.
---
You have the name; now resolve it. No kubectl, no API - just `DNS`:
`$ nslookup metrics.observability.svc.cluster.local` (or, if you prefer libc
resolution, `$ getent hosts metrics.observability.svc.cluster.local`).

Back comes a `ClusterIP`. Notice what just happened: an account that cannot
`list` a single `Service` over the API just located one by name, on a plane
where `RBAC` was never in the way.
