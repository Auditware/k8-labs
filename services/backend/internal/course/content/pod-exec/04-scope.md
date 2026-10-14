---
kind: concept
ey: CONCEPT 3
eyt: What the grant really means
chip:
  label: KEY IDEA
  tone: danger
title: exec on a namespace is exec as every pod in it
aha: It reads on paper like a debugging convenience. It is really a master key to every identity that happens to run in that namespace.
diagram:
  caption: one grant, every identity in the namespace
  shapes:
    - {type: box, x: 60, y: 116, w: 165, h: 66, label: 'pods/exec', sub: 'granted on a namespace', tone: danger, delay: 0}
    - {type: wire, from: {x: 225, y: 149}, to: {x: 330, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 330, y: 92, w: 130, h: 50, label: 'pod A token', delay: 0.7}
    - {type: box, x: 330, y: 158, w: 130, h: 50, label: 'pod B token', delay: 0.9}
    - {type: label, x: 520, y: 150, text: 'all reachable', tone: danger, delay: 1.1}
---
Permission to `exec` is granted per namespace, on `pods` in general - not per
pod. So a grant that looks like "let this account debug pods in `ops`" is really
"let this account become any identity running in `ops`".

That turns a modest-looking grant into a survey problem: whoever holds it will
list the pods, look at which service accounts they run as, and pick the most
powerful one. The account you were handed is weak. The account you can *reach*
may not be.
