---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: The powerful pod is already running on your node, its key already on the disk. You are just the one who thought to pick it up.
revealLabel: how to start
diagram:
  caption: harvest metrics-agent's token, read ops
  shapes:
    - {type: box, x: 45, y: 116, w: 165, h: 66, label: 'workstation', sub: 'reads the node', tone: danger, delay: 0}
    - {type: wire, from: {x: 210, y: 149}, to: {x: 310, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 310, y: 116, w: 165, h: 66, label: 'collector token', sub: 'off the disk', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 475, y: 149}, to: {x: 525, y: 110}, tone: accent, delay: 0.95}
    - {type: box, x: 500, y: 60, w: 115, h: 50, label: 'flag', sub: 'ops', tone: accent, delay: 1.15}
---
Your session drops you into a pod called `workstation` in the `lab` namespace.
Your own `station` account reads nothing (`$ kubectl auth can-i --list`), but the
node's filesystem is mounted at `/host`.

A pod named `metrics-agent` runs on the node as a `collector` account that can
read the `flag` Secret in `ops`. Its `token` is on the disk - find the token
files under `/host/var/lib/kubelet/pods` with `$ find`, then use
`$ kubectl --token` with each to see which can read secrets. Carry the powerful
one and read the `flag` in `ops`, then decode it. Your own account never gained a
thing - you simply picked up a stronger one off the floor.
