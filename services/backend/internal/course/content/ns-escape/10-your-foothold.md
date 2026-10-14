---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: Everything you need is already inside your shell. The task is not to break in - it is to notice what you were handed.
revealLabel: how to start
diagram:
  caption: one of these arrows should not exist
  shapes:
    - {type: box, x: 55, y: 60, w: 200, h: 66, label: 'pod foothold', sub: 'in namespace dev', delay: 0}
    - {type: wire, from: {x: 155, y: 126}, to: {x: 155, y: 175}, delay: 0.3}
    - {type: box, x: 55, y: 175, w: 200, h: 66, label: 'ServiceAccount', sub: 'analyst', tone: accent, delay: 0.5}
    - {type: wire, from: {x: 255, y: 208}, to: {x: 385, y: 150}, tone: danger, dashed: true, delay: 0.8}
    - {type: box, x: 385, y: 118, w: 200, h: 66, label: 'secret flag', sub: 'in namespace production', tone: danger, delay: 1.1}
---
Your session drops you into a pod called `foothold`, in the `dev` namespace,
running as the `analyst` service account with its token mounted.

Somewhere in the `production` namespace there is a Secret named `flag`. Your job
is to read it. Start by asking what analyst is allowed to do -
`$ kubectl auth can-i --list` - then pay very close attention to **how** that
permission was granted rather than just what it allows. When the cross-namespace
answer comes back yes, `$ kubectl get secrets -n production` lists the Secret you
were never meant to reach - now go read its value and decode it, and the boundary
was only ever a suggestion.
