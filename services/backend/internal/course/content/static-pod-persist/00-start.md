---
kind: concept
ey: START
eyt: Static Pod Persistence
tag: what you'll learn
title: You can write one directory. That is enough to run a pod nobody approved.
aha: Every control in Kubernetes lives at the front door - the API server. This lab walks a pod in through a side entrance the kubelet leaves open.
revealLabel: why this matters
diagram:
  caption: one door skips every check
  shapes:
    - {type: box, x: 55, y: 115, w: 175, h: 66, label: 'you', sub: 'write a directory', tone: danger, delay: 0}
    - {type: wire, from: {x: 230, y: 148}, to: {x: 345, y: 148}, tone: danger, delay: 0.4}
    - {type: box, x: 345, y: 115, w: 210, h: 66, label: 'kubelet runs it', sub: 'no checks applied', tone: danger, delay: 0.7}
---
You have a shell in a pod that can write to one directory on the node, and little
else. It cannot read the flag. But that directory is one the kubelet watches -
and anything you put there, it runs.

Over the next few screens we will show why a pod started that way skips every
check the cluster has, letting you run one with powers your account was never
granted.
