---
kind: concept
ey: DEEP DIVE
eyt: The cluster-scoped twin
chip:
  label: THE TRAP
  tone: danger
title: The cluster-wide versions look almost identical
aha: Two words, four extra letters, and one of them quietly means every namespace that exists - including the ones created tomorrow.
revealLabel: the takeaway
diagram:
  caption: same Role, two very different reaches
  shapes:
    - {type: box, x: 55, y: 70, w: 230, h: 60, label: 'RoleBinding', sub: 'grants in one namespace', tone: accent, delay: 0}
    - {type: box, x: 55, y: 168, w: 230, h: 60, label: 'ClusterRoleBinding', sub: 'grants in every namespace', tone: danger, delay: 0.3}
    - {type: wire, from: {x: 285, y: 100}, to: {x: 380, y: 100}, tone: accent, delay: 0.6}
    - {type: wire, from: {x: 285, y: 198}, to: {x: 380, y: 198}, tone: danger, delay: 0.7}
    - {type: box, x: 380, y: 70, w: 200, h: 60, label: 'dev only', delay: 0.9}
    - {type: box, x: 380, y: 168, w: 200, h: 60, label: 'dev + production', sub: 'and every future one', tone: danger, delay: 1.0}
---
Kubernetes also has cluster-scoped versions of both pieces. A `ClusterRole` is the
same kind of permission list, but it is not owned by any namespace. A
`ClusterRoleBinding` grants one **across the entire cluster**.

Here is the part worth slowing down for. A ClusterRole is not dangerous by
itself - you can bind one with an ordinary `RoleBinding`, and the grant stays
inside that namespace. What decides the blast radius is **which binding you
used**, not which role.
