---
kind: concept
ey: CONCEPT 5
eyt: The superuser group
chip:
  label: KEY IDEA
  tone: danger
title: One group is permanently bound to cluster-admin
aha: system:masters is not a hardcoded backdoor - it is an ordinary cluster-admin binding. What makes it dangerous is that the cluster re-creates it if you delete it, and nothing scopes it.
diagram:
  caption: system:masters -> cluster-admin, a binding you cannot remove
  shapes:
    - {type: box, x: 60, y: 110, w: 175, h: 66, label: 'request', sub: 'group system:masters', tone: danger, delay: 0}
    - {type: wire, from: {x: 235, y: 143}, to: {x: 355, y: 143}, tone: danger, delay: 0.4}
    - {type: box, x: 355, y: 110, w: 210, h: 66, label: 'cluster-admin', sub: 'binding auto-reconciled', tone: danger, delay: 0.7}
    - {type: seal, cx: 300, cy: 210, r: 20, broken: true, delay: 1.0}
---
The cluster ships with a default `ClusterRoleBinding` that binds the group
`system:masters` to the `cluster-admin` `ClusterRole` - so any request carrying
that group is a cluster-admin. It is the group behind the cluster-admin
credential your provider hands you.

This is still RBAC, but it is RBAC you cannot take back: the API server treats
that bootstrap binding as protected and re-creates it if you delete it, and
because it is cluster-scoped, no Role and no namespace can narrow it. So an
identity that can `impersonate` a `group` and names this one is not escalating
within its own grants - it is putting on the one identity the cluster will
always trust.
