---
kind: concept
ey: START
eyt: First Contact
tag: what you'll learn
title: You woke up inside a pod. Inventory before you move.
aha: A good operator does not reach for the network first. They empty their pockets - the pod already handed them more than they think.
revealLabel: why this matters
diagram:
  caption: everything you need first is already inside the pod
  shapes:
    - {type: box, x: 60, y: 116, w: 200, h: 64, label: 'you', sub: 'a fresh shell', delay: 0}
    - {type: wire, from: {x: 260, y: 148}, to: {x: 370, y: 148}, delay: 0.4}
    - {type: box, x: 370, y: 116, w: 210, h: 64, label: 'the pod', sub: 'token, env, mounts', tone: accent, delay: 0.7}
---
You just landed a shell in a pod. No exploit yet, no lateral move - just a
prompt. The instinct is to start scanning the network. Resist it.

A running pod is handed a surprising amount at birth: its own identity, its
configuration, its mounts. Over the next few screens you will take inventory of
what this pod already holds - and find that the first secret was never on the
network at all.
