---
kind: concept
ey: CONCEPT 2
eyt: What node access means
chip:
  label: KEY IDEA
  tone: danger
title: Reading the node is reading everyone who runs on it
aha: You did not compromise the powerful pod. You compromised the shelf its keys were resting on, along with everyone else's.
diagram:
  caption: your reach is the strongest token present
  shapes:
    - {type: box, x: 55, y: 116, w: 165, h: 66, label: 'node access', sub: 'read the disk', tone: danger, delay: 0}
    - {type: wire, from: {x: 220, y: 149}, to: {x: 330, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 330, y: 92, w: 150, h: 44, label: 'weak token', delay: 0.7}
    - {type: box, x: 330, y: 158, w: 175, h: 44, label: 'powerful token', tone: danger, delay: 0.9}
---
Your foothold has a `hostPath` mount of the node's filesystem, so those `token`
files are readable. That changes the question entirely. Your own `ServiceAccount` might be
powerless, but the pods scheduled beside you may not be - a logging agent, a
metrics collector, a controller.

So your effective access is not what your account holds; it is what the *most
powerful* pod on this node holds. Node access collapses every identity on the
machine down to whoever you choose to become. The task is to find the strongest
one.
