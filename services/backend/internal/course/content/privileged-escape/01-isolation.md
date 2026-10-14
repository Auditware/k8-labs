---
kind: concept
ey: CONCEPT 1
eyt: What isolates a container
tag: foundation
title: A container is isolation built from namespaces and dropped powers
aha: The container wall is not one thing. It is a stack of small restrictions, each of which can be individually switched off.
diagram:
  caption: isolation is a set of switches, not a wall
  shapes:
    - {type: box, x: 60, y: 108, w: 210, h: 92, label: 'container', sub: 'own view + fewer powers', delay: 0}
    - {type: wire, from: {x: 270, y: 154}, to: {x: 380, y: 154}, delay: 0.5}
    - {type: box, x: 380, y: 120, w: 190, h: 66, label: 'the node', sub: 'the real machine', tone: accent, delay: 0.8}
---
A container is isolated by two mechanisms. A `namespace` gives it its own view of
some resource - its own filesystem root, its own process list, its own network -
so it cannot see the node's. And the kernel drops most `capability` bits, the
fine-grained privileges that let a process do powerful things like access raw
devices or trace other processes.

Neither is a wall so much as a setting. Turn a namespace off and the pod shares
the node's view of that resource. Restore the capabilities and it can do what root
on the node can do. This lab is about switching both off.
