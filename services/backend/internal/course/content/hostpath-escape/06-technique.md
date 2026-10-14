---
kind: concept
ey: CONCEPT 4
eyt: Finding the way out
tag: technique
title: Read your own mounts, then walk the node
aha: The exit is not hidden. It is listed plainly in your own pod, if you think to look at how you are mounted.
diagram:
  caption: your mounts name the door
  shapes:
    - {type: box, x: 60, y: 118, w: 175, h: 64, label: 'mount', sub: 'lists your volumes', delay: 0}
    - {type: wire, from: {x: 235, y: 150}, to: {x: 350, y: 150}, tone: accent, delay: 0.4}
    - {type: box, x: 350, y: 118, w: 210, h: 64, label: 'hostPath at /host', sub: 'the way out', tone: danger, delay: 0.7}
---
The first move after landing in any container is to look at how it is built.
`$ mount` and `$ ls /` show what is attached to your pod - and a directory like
`/host` holding the node's whole filesystem stands out immediately.

From there it is ordinary navigation. `$ ls /host` is the node's root; the node's
own files sit under it exactly where they would on the host. Explore it the way
you would any machine you had just landed on - the interesting things are the
files the node keeps for itself.
