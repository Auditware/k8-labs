---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: The powerful role is already sitting on the shelf, and you have been handed the one permission that lets you take it down for yourself.
revealLabel: how to start
diagram:
  caption: bind secret-admin to yourself, then read
  shapes:
    - {type: box, x: 45, y: 116, w: 155, h: 66, label: 'foothold', sub: 'sa-binder', delay: 0}
    - {type: wire, from: {x: 200, y: 149}, to: {x: 300, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 300, y: 116, w: 175, h: 66, label: 'bind secret-admin', sub: 'to yourself', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 475, y: 149}, to: {x: 535, y: 110}, tone: accent, delay: 0.95}
    - {type: box, x: 510, y: 60, w: 110, h: 50, label: 'flag', sub: 'production', tone: accent, delay: 1.15}
---
Your session drops you into a pod called `foothold` in the `dev` namespace,
running as the `sa-binder` service account. List your grants with
`$ kubectl auth can-i --list -n production` and read them carefully. **Mind the
namespace:** your powers live in `production`, so a bare `--list` (or `-A`) checks
`dev`, where you hold nothing, and looks like an account with no access. That is
not a broken lab - point it at `production` and the grants appear: you can create
`RoleBinding` objects there, and you hold `bind` on one `ClusterRole`.

The Secret named `flag` lives in `production` and you cannot read it yet. Create
a `RoleBinding` in `production` that grants that bindable role to your own
`sa-binder` identity - `$ kubectl create rolebinding` builds one. If you name a
role you were not given `bind` on, the API server refuses it, so use the right
one. Then read the `flag` and decode it. You wrote yourself the access.
