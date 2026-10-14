---
kind: concept
ey: START
eyt: Node Token Harvest
tag: what you'll learn
title: Once you can read the node, every pod's identity is yours.
aha: An escape is not the end of the story - it is the beginning. The node is where the cluster keeps everyone's keys in one drawer.
revealLabel: why this matters
diagram:
  caption: the node holds every pod's credentials
  shapes:
    - {type: box, x: 55, y: 116, w: 175, h: 66, label: 'you', sub: 'read the node', tone: danger, delay: 0}
    - {type: wire, from: {x: 230, y: 149}, to: {x: 340, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 340, y: 116, w: 215, h: 66, label: 'tokens on disk', sub: 'every pod, one drawer', tone: danger, delay: 0.7}
---
You have a shell in a pod that can read the node's filesystem, but your own
service account can read nothing useful. In the earlier escape labs, reaching the
node was the goal. Here it is the setup.

Over the next few screens we will show why node access quietly hands you every
identity running on that node - and how to pick the most powerful one and use it
to read what you cannot.
