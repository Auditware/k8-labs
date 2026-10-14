---
kind: concept
ey: CONCEPT 3
eyt: The second switch
tag: the machinery
title: hostPID makes the node's processes yours to see
aha: The one thing still hiding the node was your private list of processes. hostPID hands you the node's list instead - including its very first process.
diagram:
  caption: the node's init becomes your PID 1
  shapes:
    - {type: box, x: 55, y: 116, w: 175, h: 66, label: 'hostPID', sub: 'share node processes', tone: danger, delay: 0}
    - {type: wire, from: {x: 230, y: 149}, to: {x: 345, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 345, y: 116, w: 210, h: 66, label: 'node init = PID 1', sub: 'visible in your pod', tone: danger, delay: 0.7}
---
The second setting is `hostPID`. Normally a pod has its own process `namespace`,
so it sees only its own processes. With `hostPID`, the pod shares the node's
process namespace: it sees every process on the node, and the node's init - the
first process a machine starts - appears as `PID 1` inside your pod.

That matters because a process is a handle to more than a program. Every running
process exposes its own filesystem root, and the node's init is rooted at the
node's filesystem. Being able to see `PID 1`, with the power `privileged` gave
you, is the whole escape.
