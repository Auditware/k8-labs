---
kind: concept
ey: DEEP DIVE
eyt: Putting it together
chip:
  label: THE TRAP
  tone: danger
title: A SUID program that shells out unsanitized input runs your command as root
aha: Neither half is remarkable on its own. A SUID bit is normal. Shelling out is normal. It is the two in the same binary that hands you the machine.
revealLabel: the takeaway
diagram:
  caption: borrowed root plus obeyed punctuation
  shapes:
    - {type: box, x: 50, y: 70, w: 220, h: 58, label: 'SUID root', sub: 'runs as root', tone: warn, delay: 0}
    - {type: box, x: 50, y: 168, w: 220, h: 58, label: 'shells out your input', sub: 'obeys your punctuation', tone: warn, delay: 0.3}
    - {type: wire, from: {x: 270, y: 99}, to: {x: 360, y: 140}, tone: danger, delay: 0.6}
    - {type: wire, from: {x: 270, y: 197}, to: {x: 360, y: 156}, tone: danger, delay: 0.7}
    - {type: box, x: 360, y: 120, w: 210, h: 58, label: 'your command, as root', tone: danger, delay: 1.0}
---
Line up the two ideas. The binary runs with root's `euid` because of the `SUID`
bit. It passes your argument to a shell without sanitizing it, because of
`command injection`. So the command you smuggle in through that argument runs
with root's `euid`.

That is the whole exploit. You are not breaking the program - you are using it
exactly as built, feeding it an input its author never pictured, and the shell
does the rest.
