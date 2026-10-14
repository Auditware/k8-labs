---
kind: concept
ey: CONCEPT 3
eyt: The lent identity
tag: the machinery
title: The API server will act as someone else, if you are allowed to ask
aha: It exists for good reasons - a gateway proving a real user is behind a request. Handed to the wrong account, it becomes a costume rack.
diagram:
  caption: your request, carried out under another name
  shapes:
    - {type: box, x: 55, y: 115, w: 175, h: 66, label: 'sa-imper', sub: 'kubectl --as=bob', delay: 0}
    - {type: wire, from: {x: 230, y: 148}, to: {x: 350, y: 148}, tone: danger, delay: 0.4}
    - {type: box, x: 350, y: 100, w: 225, h: 96, label: 'API server', sub: 'authorises as bob, not you', tone: danger, delay: 0.7}
---
Kubernetes has a built-in feature called `impersonate`. A caller permitted to
use it can attach `$ kubectl --as` to a request, and the API server evaluates
that request as the *named* identity instead of the caller's own.

This is how a front proxy or gateway forwards a request on a real user's behalf.
The permission to do it is an ordinary RBAC verb, `impersonate`, granted on the
`users` or `groups` resource. Whoever holds it does not gain a specific power -
they gain the ability to *choose* which identity the cluster sees.
