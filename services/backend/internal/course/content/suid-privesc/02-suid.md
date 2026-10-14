---
kind: concept
ey: CONCEPT 2
eyt: Borrowed identity
chip:
  label: KEY
  tone: accent
title: A SUID program runs as its owner, not as you
aha: It is a key cut to open one specific door on someone else's behalf. Handy when the program is careful. A loaded gun when it is not.
diagram:
  caption: the euid becomes the file's owner on exec
  shapes:
    - {type: box, x: 55, y: 115, w: 150, h: 64, label: 'you', sub: 'uid 10001', delay: 0}
    - {type: wire, from: {x: 205, y: 147}, to: {x: 285, y: 147}, delay: 0.4}
    - {type: box, x: 285, y: 105, w: 190, h: 84, label: 'SUID binary', sub: 'owner root · bit set', tone: warn, delay: 0.6}
    - {type: wire, from: {x: 475, y: 147}, to: {x: 533, y: 147}, tone: danger, delay: 0.9}
    - {type: box, x: 533, y: 115, w: 84, h: 64, label: 'euid 0', tone: danger, delay: 1.1}
---
Some programs need more power than the user running them - `passwd` has to edit a
root-owned file so you can change your own password. Unix solves this with the
`SUID` bit. When a file has it set, running the program switches the process's
effective identity - its `euid` - to the file's **owner**, for as long as it runs.

A `SUID`-root binary therefore executes as root no matter who launches it -
`$ ls -l /usr/local/bin/backup-tool` shows the `s` in its permissions that says so. That is
exactly the point, and exactly the danger: for those few milliseconds, whatever
the program does, it does as root.
