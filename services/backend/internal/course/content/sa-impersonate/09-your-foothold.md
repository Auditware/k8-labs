---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: Everything you need is already on your token. The task is not to break in - it is to notice which verb you were handed and aim it.
revealLabel: how to start
diagram:
  caption: the account is powerless; the verb is not
  shapes:
    - {type: box, x: 55, y: 60, w: 200, h: 66, label: 'pod foothold', sub: 'in namespace dev', delay: 0}
    - {type: wire, from: {x: 155, y: 126}, to: {x: 155, y: 175}, delay: 0.3}
    - {type: box, x: 55, y: 175, w: 200, h: 66, label: 'sa-imper', sub: 'can impersonate only', tone: accent, delay: 0.5}
    - {type: wire, from: {x: 255, y: 208}, to: {x: 385, y: 150}, tone: danger, dashed: true, delay: 0.8}
    - {type: box, x: 385, y: 118, w: 200, h: 66, label: 'secret flag', sub: 'in namespace production', tone: danger, delay: 1.1}
---
Your session drops you into a pod called `foothold` in the `dev` namespace,
running as the `sa-imper` service account with its token mounted.

In the `production` namespace there is a Secret named `flag` you cannot read as
yourself. Start by listing your grants - `$ kubectl auth can-i --list` - and
confirm the direct path is closed: `$ kubectl get secrets -n production`. Then
recall which verb you *do* hold, and which group turns it into a superuser. The
API server exposes impersonation through `--as` and `--as-group`; assemble the
one request that reads the Secret, then decode its value. The account never
gained a read grant - it just stopped being the one asking.
