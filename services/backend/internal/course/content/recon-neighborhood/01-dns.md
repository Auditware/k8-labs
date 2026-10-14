---
kind: concept
ey: CONCEPT 1
eyt: A different plane
tag: technique
title: DNS never checks your permissions
aha: RBAC guards the API server. It has nothing to do with name resolution - a separate service that answers everyone.
---
Kubernetes runs a cluster `DNS` service (CoreDNS) that resolves every `Service`
to its address. Crucially, it does not consult `RBAC`: DNS is a different plane,
and it answers any pod that can send it a query.

So the fact that you cannot `list` services over the API says nothing about
whether you can resolve and reach them. You only need a name to look up - and the
pod you compromised already knows several.
