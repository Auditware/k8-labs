---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: Both switches are already flipped for you. The task is to recognise them and walk through the door they leave open.
revealLabel: how to start
diagram:
  caption: through PID 1's root to the node
  shapes:
    - {type: box, x: 55, y: 116, w: 180, h: 66, label: 'workstation', sub: 'privileged + hostPID', tone: danger, delay: 0}
    - {type: wire, from: {x: 235, y: 149}, to: {x: 350, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 350, y: 116, w: 205, h: 66, label: '/proc/1/root', sub: 'flag under /opt', tone: danger, delay: 0.8}
---
Your session drops you into a pod called `workstation` in the `lab` namespace.
There is no token and nothing to read over the API. Look at how the pod is
built - `$ id` shows you are root in the container, and the pod is `privileged`
with `hostPID`.

That means the node's init is `PID 1` in your view, and `/proc/1/root` is the
node's filesystem. Confirm it - `$ cat /proc/1/root/etc/hostname` returns the
node's name, not your pod's. The flag is a root-owned file under the node's
`/opt`; read it through `/proc/1/root` and decode nothing - it is plain text. The
container was never really holding you.
