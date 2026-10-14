---
kind: concept
ey: START
eyt: Pod Exec Lateral Movement
tag: what you'll learn
title: You can open a shell in one pod. That is the whole attack.
aha: You are not going to read the secret. Someone else already can - and you can become them by standing inside their pod.
revealLabel: why this matters
diagram:
  caption: you cannot read it; the pod next door can
  shapes:
    - {type: box, x: 50, y: 115, w: 165, h: 66, label: 'you', sub: 'can read nothing', delay: 0}
    - {type: wire, from: {x: 215, y: 148}, to: {x: 320, y: 148}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 320, y: 115, w: 150, h: 66, label: 'a pod', sub: 'that can', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 470, y: 148}, to: {x: 525, y: 110}, tone: danger, delay: 0.9}
    - {type: box, x: 500, y: 60, w: 110, h: 50, label: 'secret', tone: accent, delay: 1.1}
---
You have a shell in a cluster. Your own identity can read nothing worth reading.
But you have been granted one narrow-sounding power - the ability to run a
command inside other pods.

Over the next few screens we will show why that single power lets you put on the
identity of any pod you can reach, and read whatever it can read.
