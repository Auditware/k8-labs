---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: The pod you need is already running and already privileged. Your job is to walk into it and pick up what it left on the floor.
revealLabel: how to start
diagram:
  caption: exec across, then read what ci can read
  shapes:
    - {type: box, x: 45, y: 118, w: 165, h: 66, label: 'foothold', sub: 'dev, exec only', delay: 0}
    - {type: wire, from: {x: 210, y: 151}, to: {x: 315, y: 151}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 315, y: 118, w: 150, h: 66, label: 'ci-runner', sub: 'ops, has token', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 465, y: 151}, to: {x: 525, y: 110}, tone: accent, delay: 0.95}
    - {type: box, x: 500, y: 60, w: 115, h: 50, label: 'flag', sub: 'production', tone: accent, delay: 1.15}
---
Your session drops you into a pod called `foothold` in the `dev` namespace. Its
`ServiceAccount` can `exec` into pods in the `ops` namespace and nothing else.

In `ops` there is a pod named `ci-runner`, running as an account that can read
the Secret named `flag` in `production`. Confirm your own reach with
`$ kubectl auth can-i --list -n ops`. **The namespace is the whole trick here:**
your grant is scoped to `ops`, so a bare `--list` (which checks only your own
`dev` namespace) or an `-A` / all-namespaces check both report nothing - that is
not a broken lab, it means you have to name the namespace. Find the target with
`$ kubectl get pods -n ops`, then exec into it. Inside, you are `ci-runner` - its `token` is mounted and
`kubectl` will use it automatically. From there, read the `flag` in `production`
and decode it. You never gained a read grant; you borrowed one.
