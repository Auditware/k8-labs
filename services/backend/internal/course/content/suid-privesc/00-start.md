---
kind: concept
ey: START
eyt: SUID Privilege Escalation
tag: what you'll learn
title: One file trusts its input too much, and it is allowed to be root.
aha: The bug is not exotic. A program takes a filename and hands it to a shell. That is a mistake people make on the first day and ship on the last.
revealLabel: why this matters
diagram:
  caption: an ordinary user, one over-trusted program, a root shell
  shapes:
    - {type: box, x: 55, y: 112, w: 160, h: 66, label: 'you', sub: 'an ordinary user', delay: 0}
    - {type: wire, from: {x: 215, y: 145}, to: {x: 330, y: 145}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 330, y: 105, w: 175, h: 80, label: 'backup-tool', sub: 'runs as root', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 505, y: 145}, to: {x: 576, y: 145}, tone: danger, delay: 1.0}
    - {type: seal, cx: 596, cy: 145, r: 16, broken: true, delay: 1.1}
---
You have a normal shell on a machine, as a normal user. You cannot read the
secret on it, and you should not be able to.

Over the next few screens we will build up exactly why one particular program
lets you become root anyway - then you will go and do it. Nothing here is a
memory-corruption trick; it is a design mistake you can read in one line.
