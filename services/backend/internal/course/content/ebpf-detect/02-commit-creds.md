---
kind: concept
ey: CONCEPT 2
eyt: The one choke point
chip:
  label: KEY
  tone: accent
title: Every credential change flows through one kernel function
aha: If you wanted to watch every privilege change on a machine and only had one place to stand, this is it. You do not have to guess how someone became root - they all walk through the same door.
diagram:
  caption: many ways up, one function that installs the result
  shapes:
    - {type: box, x: 40, y: 60, w: 175, h: 46, label: 'SUID exec', sub: '', tone: warn, delay: 0}
    - {type: box, x: 40, y: 124, w: 175, h: 46, label: 'setuid()', sub: '', tone: warn, delay: 0.3}
    - {type: box, x: 40, y: 188, w: 175, h: 46, label: 'su / sudo', sub: '', tone: warn, delay: 0.6}
    - {type: wire, from: {x: 215, y: 83}, to: {x: 335, y: 140}, delay: 0.8}
    - {type: wire, from: {x: 215, y: 147}, to: {x: 335, y: 147}, delay: 0.9}
    - {type: wire, from: {x: 215, y: 211}, to: {x: 335, y: 154}, delay: 1.0}
    - {type: box, x: 335, y: 116, w: 230, h: 62, label: 'commit_creds()', sub: 'installs the new credentials', tone: danger, delay: 1.2}
---
Underneath all of it, the kernel installs credentials in exactly one place. A
`SUID` exec, a `setuid()` call, `su`, `sudo` - however a process ends up with new
identity, the kernel calls `commit_creds` to make it official.

One function, every privilege change. That is what makes it the perfect thing to
watch: you do not need to anticipate the trick, only to sit on the choke point
every trick has to pass through.
