---
kind: concept
ey: CONCEPT 4
eyt: Getting the loot out
tag: technique
title: Write the manifest, let it run, read what it leaves behind
aha: You do not need a shell in the static pod. You send it in with instructions and a place to leave the answer, then collect it.
diagram:
  caption: the static pod drops loot where you can read it
  shapes:
    - {type: box, x: 45, y: 116, w: 165, h: 66, label: 'static pod', sub: 'mounts node /', tone: danger, delay: 0}
    - {type: wire, from: {x: 210, y: 149}, to: {x: 305, y: 149}, tone: accent, delay: 0.4}
    - {type: box, x: 305, y: 116, w: 150, h: 66, label: 'loot file', sub: 'in shared dir', tone: accent, delay: 0.7}
    - {type: wire, from: {x: 455, y: 149}, to: {x: 545, y: 149}, delay: 0.9}
    - {type: box, x: 545, y: 116, w: 75, h: 66, label: 'you', delay: 1.1}
---
Write a pod manifest into the kubelet's directory - the `static pod` should mount
the node's `/`, read the file you are after, and write it back somewhere you can
reach. The directory you can already write is a fine drop point: mount it in the
static pod too, and have it copy the loot there.

Give the kubelet a few seconds to notice the file and start the pod, then read the
loot from your own mounted directory. No interactive shell inside the static pod
is needed - it does the read for you and leaves the answer where you asked.
