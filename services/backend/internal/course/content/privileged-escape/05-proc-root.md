---
kind: concept
ey: CONCEPT 4
eyt: The door itself
tag: the machinery
title: /proc/1/root is the node's filesystem, if you are allowed to follow it
aha: Linux quietly publishes every process's root directory under /proc. Follow the node init's, and you are standing in the node's filesystem.
diagram:
  caption: PID 1's root is the node's root
  shapes:
    - {type: box, x: 55, y: 116, w: 180, h: 66, label: '/proc/1/root', sub: 'node init root', tone: danger, delay: 0}
    - {type: wire, from: {x: 235, y: 149}, to: {x: 350, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 350, y: 116, w: 205, h: 66, label: 'node files', sub: 'read them directly', tone: danger, delay: 0.7}
---
The kernel exposes each process's root directory at `/proc/<pid>/root`. Follow
another process's and you see the filesystem as that process sees it. Because
`hostPID` made the node's init `PID 1`, the path `/proc/1/root` is the node's own
root filesystem - and reading a file there, like `/proc/1/root/etc/hostname`,
reads the node's copy.

Following into another process's root needs the power to reach into it, which is
exactly what `privileged` restored. Neither switch alone gets you there:
`privileged` without `hostPID` cannot see `PID 1`; `hostPID` without `privileged`
can see it but is denied the read. Together, `/proc/1/root` is an open door.
