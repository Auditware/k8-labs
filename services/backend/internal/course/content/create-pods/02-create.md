---
kind: concept
ey: CONCEPT 2
eyt: The create verb
tag: the machinery
title: create pods is permission to run code, as your chosen identity
aha: It sounds like a scheduling chore. It is really the right to boot a program that already holds someone else's credentials.
diagram:
  caption: the verb that turns YAML into a running identity
  shapes:
    - {type: box, x: 55, y: 116, w: 180, h: 66, label: 'create pods', sub: 'in a namespace', tone: danger, delay: 0}
    - {type: wire, from: {x: 235, y: 149}, to: {x: 350, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 350, y: 116, w: 215, h: 66, label: 'running pod', sub: 'as any account there', tone: danger, delay: 0.7}
---
The RBAC verb `create pods` lets you submit a pod for the namespace to run. On
its own that sounds operational. But combine it with the previous screen: you
also choose the pod's `serviceAccountName`, and no rule stops you naming an
account more powerful than your own.

So `create pods` is not "can schedule work". It is "can run a process holding any
identity in this namespace". The pod does the talking to the API; you just decide
whose voice it uses.
