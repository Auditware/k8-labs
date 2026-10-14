---
kind: concept
ey: CONCEPT 1
eyt: Who you are
tag: foundation
title: A process runs as a user, and that user is checked on every action
aha: Think of a keycard. The building does not ask who you are at each door - it reads the card the door was opened with. A process carries its card the same way.
diagram:
  caption: the kernel checks the uid, not your intentions
  shapes:
    - {type: box, x: 60, y: 115, w: 170, h: 64, label: 'your shell', sub: 'uid 10001', delay: 0}
    - {type: wire, from: {x: 230, y: 147}, to: {x: 320, y: 147}, delay: 0.4}
    - {type: box, x: 320, y: 115, w: 150, h: 64, label: 'open a file', tone: accent, delay: 0.6}
    - {type: wire, from: {x: 470, y: 147}, to: {x: 525, y: 147}, tone: accent, delay: 0.8}
    - {type: box, x: 525, y: 115, w: 92, h: 64, label: 'kernel', sub: 'allowed?', delay: 1.0}
---
Every process on the machine runs as some `user`, identified by a number - its
`uid`. Your shell runs as an unprivileged user - `$ id` shows which one. Root is `uid` 0, and the kernel
grants it everything.

When a process tries to read a file or run a command, the kernel checks the
process's `uid` against the permissions - not who typed the command, not what
the program is named. The identity a process carries is the whole story. So the
interesting question becomes: can a process ever carry a different one than the
user who started it?
