---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: You were handed the door already open. The whole task is to notice it and step through.
revealLabel: how to start
diagram:
  caption: through /host to the node's own files
  shapes:
    - {type: box, x: 55, y: 116, w: 175, h: 66, label: 'workstation', sub: 'mounts /host', delay: 0}
    - {type: wire, from: {x: 230, y: 149}, to: {x: 345, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 345, y: 116, w: 210, h: 66, label: 'node filesystem', sub: 'flag under /opt', tone: danger, delay: 0.8}
---
Your session drops you into a pod called `workstation` in the `lab` namespace.
There is no useful service account and nothing to read over the API. But look at
how the pod is mounted.

Run `$ mount` or `$ ls /host` - the node's root filesystem is mounted into your
pod. The flag is a root-owned file the node keeps to itself, planted somewhere
under the node's `/opt`. Walk the mounted filesystem, find it, and read it. No
permission was granted and none was needed - the container wall was never really
there.
