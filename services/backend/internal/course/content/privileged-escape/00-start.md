---
kind: concept
ey: START
eyt: Privileged Pod Breakout
tag: what you'll learn
title: Your container has been handed the keys to the machine under it.
aha: A privileged container is not a locked room with a strong door. It is a room whose walls were never actually built.
revealLabel: why this matters
diagram:
  caption: the box is open at the bottom
  shapes:
    - {type: box, x: 55, y: 70, w: 210, h: 58, label: 'your pod', sub: 'privileged', tone: danger, delay: 0}
    - {type: wire, from: {x: 160, y: 128}, to: {x: 160, y: 176}, tone: danger, delay: 0.4}
    - {type: box, x: 55, y: 176, w: 210, h: 58, label: 'the node', sub: 'yours to read', tone: danger, delay: 0.8}
    - {type: label, x: 380, y: 150, text: 'no wall in between', tone: danger, delay: 1.1}
---
You have a shell in a pod again, and again the flag is a root-owned file on the
node, not something in the cluster. But this pod has no convenient hostPath
mount. Instead it was given two settings that, together, let you walk out
deliberately.

Over the next few screens we will build up what those two settings do, and how
they turn an ordinary shell into a read of the node's own filesystem.
