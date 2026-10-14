---
kind: concept
ey: START
eyt: hostPath Node Escape
tag: what you'll learn
title: The flag is not in the cluster. It is on the machine underneath it.
aha: You will not ask the API for anything. You will simply step out of your container and read the floor it was standing on.
revealLabel: why this matters
diagram:
  caption: the prize sits below the container line
  shapes:
    - {type: box, x: 55, y: 70, w: 200, h: 60, label: 'your container', sub: 'what you can see', delay: 0}
    - {type: wire, from: {x: 155, y: 130}, to: {x: 155, y: 178}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 55, y: 178, w: 200, h: 60, label: 'the node', sub: 'where the flag is', tone: danger, delay: 0.8}
    - {type: label, x: 360, y: 150, text: 'one mount joins them', tone: danger, delay: 1.1}
---
You have a shell in a pod. This time there is no secret to read over the API, no
service account to borrow. The flag is a file sitting on the node - the machine
your container runs on - owned by root, invisible to a normal container.

Over the next few screens we will show why one careless line in your pod's
definition hands you the node's entire filesystem, and how to walk out of the
container and take what is there.
