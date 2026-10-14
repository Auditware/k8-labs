---
kind: concept
ey: CONCEPT 5
eyt: Asking the cluster
tag: technique
title: You can simply ask what you are allowed to do
aha: Most systems make you guess at your own permissions. This one will read them out loud, to anyone holding the badge.
diagram:
  caption: the API answers honestly about your own access
  shapes:
    - {type: box, x: 60, y: 118, w: 200, h: 64, label: 'your shell', sub: 'kubectl auth can-i', delay: 0}
    - {type: wire, from: {x: 260, y: 150}, to: {x: 370, y: 150}, delay: 0.4}
    - {type: box, x: 370, y: 118, w: 210, h: 64, label: 'API', sub: 'checks your bindings', tone: accent, delay: 0.6}
    - {type: label, x: 315, y: 205, text: 'no permission needed to ask', delay: 0.9}
---
Kubernetes will tell you your own permissions. `$ kubectl auth can-i --list` prints
what your identity may do in a namespace, and
`$ kubectl auth can-i get secrets -n production` asks whether that reach extends
into another one. Run both - the second answer is the whole lab.

This is not a flaw - a process should be able to discover its own access. But it
does mean an attacker with a foothold never has to guess. The first move is
always to ask.
