---
kind: concept
ey: CONCEPT 1
eyt: What a container is
tag: foundation
title: A container is a narrow view of the machine, not a separate machine
aha: The wall between your container and the node is a matter of what you are shown, not a physical barrier. Widen the view and the wall is just gone.
diagram:
  caption: many containers, one shared node
  shapes:
    - {type: frame, x: 45, y: 95, w: 300, h: 120, label: 'one node', delay: 0}
    - {type: box, x: 65, y: 135, w: 85, h: 55, label: 'pod A', delay: 0.4}
    - {type: box, x: 160, y: 135, w: 85, h: 55, label: 'pod B', delay: 0.6}
    - {type: box, x: 255, y: 135, w: 75, h: 55, label: 'you', tone: danger, delay: 0.8}
    - {type: label, x: 470, y: 150, text: 'all on the same disk', delay: 1.1}
---
Your container is not a virtual machine. It is a process on the `node` - a real
host - that has been given a restricted view: its own filesystem root, its own
process list. Underneath, it shares the node's kernel and disk with every other
pod scheduled there.

That restricted view is the only thing separating your pod from the node and from
its neighbours. The node's files - other pods' data, the kubelet's credentials -
are all right there on the same disk. You just cannot see them from inside the
box. Usually.
