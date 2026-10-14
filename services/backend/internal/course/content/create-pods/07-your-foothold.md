---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: You are handed a weak account and a workshop. The trick is to build a machine in the workshop that is stronger than you are.
revealLabel: how to start
diagram:
  caption: launch as builder, read the pod's logs
  shapes:
    - {type: box, x: 45, y: 116, w: 160, h: 66, label: 'foothold', sub: 'dev, create only', delay: 0}
    - {type: wire, from: {x: 205, y: 149}, to: {x: 300, y: 149}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 300, y: 116, w: 165, h: 66, label: 'pod as builder', sub: 'in ops', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 465, y: 149}, to: {x: 525, y: 110}, tone: accent, delay: 0.95}
    - {type: box, x: 500, y: 60, w: 115, h: 50, label: 'flag', sub: 'production', tone: accent, delay: 1.15}
---
Your session drops you into a pod called `foothold` in the `dev` namespace. Its
account can create pods (and read their logs) in `ops` - confirm with
`$ kubectl auth can-i --list -n ops`. **The namespace is the whole trick here:**
your grant is scoped to `ops`, so a bare `--list` (which checks only your own
`dev` namespace) or `$ kubectl auth can-i create pods -A` (which asks about
cluster-wide access) both report nothing. That is not a broken lab - it means you
have to name the namespace. Always pass `-n ops`.

In `ops` there is a service account named `builder` that can read the `flag`
Secret in `production`. Find it (`$ kubectl get serviceaccounts -n ops`), then
launch a pod in `ops` with its `serviceAccountName` set to `builder`, built from
`k8labs-foothold:1.31` with `imagePullPolicy: Never`, running a command that
reads the `flag` and prints it. `$ kubectl run` with `--overrides` is one way to
set the account. Then read your pod's `logs` and decode the value. You never held
a read grant - your pod did.
