---
kind: concept
ey: START
eyt: Pod Creation Escalation
tag: what you'll learn
title: If you can create a pod, you can run code as someone stronger.
aha: A pod is a little machine you get to configure - including whose identity it wakes up holding. That last setting is the whole game.
revealLabel: why this matters
diagram:
  caption: you build the pod; you also pick who it is
  shapes:
    - {type: box, x: 55, y: 115, w: 175, h: 66, label: 'you', sub: 'can create pods', delay: 0}
    - {type: wire, from: {x: 230, y: 148}, to: {x: 345, y: 148}, tone: danger, delay: 0.4}
    - {type: box, x: 345, y: 108, w: 220, h: 82, label: 'a pod you build', sub: 'runs as who you choose', tone: danger, delay: 0.7}
---
You have a shell in a cluster, and one useful permission: you can create pods in
a namespace. You cannot read the secret you are after. As yourself, you never
will.

Over the next few screens we will show why the ability to launch a pod is the
ability to run your code under any identity in that namespace - and how to have
that pod hand you what you cannot read directly.
