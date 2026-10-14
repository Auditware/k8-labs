---
kind: concept
ey: START
eyt: Sidecar Injection
tag: what you'll learn
title: You can edit a running app. That is enough to plant your code inside it.
aha: You are not breaking into the app. You are adding a roommate to it - one who quietly shares everything the app was trusted with.
revealLabel: why this matters
diagram:
  caption: your container, riding inside their workload
  shapes:
    - {type: box, x: 55, y: 115, w: 175, h: 66, label: 'you', sub: 'can edit the app', delay: 0}
    - {type: wire, from: {x: 230, y: 148}, to: {x: 345, y: 148}, tone: danger, delay: 0.4}
    - {type: box, x: 345, y: 105, w: 220, h: 90, label: 'the app pod', sub: 'now with your container', tone: danger, delay: 0.7}
---
You have a shell in a cluster, and permission to change one running application.
You cannot read the secret you are after. But that application can - and you get
to decide what runs inside it.

Over the next few screens we will show how editing a workload lets you slip your
own container into it, sharing the pod's identity, and have it read what you
cannot.
