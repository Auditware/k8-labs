---
kind: concept
ey: CONCEPT 1
eyt: Namespaces
tag: foundation
title: A namespace is a folder for cluster objects
aha: Think of desks in an open-plan office. Grouping them into teams tells you who sits where. It does not stop anyone walking to another team's desk.
diagram:
  caption: one cluster, objects grouped by namespace
  shapes:
    - {type: frame, x: 40, y: 60, w: 560, h: 190, label: 'one cluster', delay: 0}
    - {type: box, x: 80, y: 110, w: 200, h: 80, label: 'namespace dev', sub: 'pods, secrets, accounts', delay: 0.3}
    - {type: box, x: 360, y: 110, w: 200, h: 80, label: 'namespace production', sub: 'pods, secrets, accounts', tone: accent, delay: 0.6}
---
Almost everything in Kubernetes lives inside a `namespace` - pods, secrets,
accounts. A namespace gives objects a place to live and lets two teams both have
a thing called `database` without colliding.

That is the entire job: **naming and grouping**. A namespace is not a wall, not a
network boundary, and not by itself a permission boundary. Whether crossing one
is allowed is a completely separate question, answered by a system we are about
to meet.
