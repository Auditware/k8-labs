---
kind: concept
ey: CONCEPT 1
eyt: A jump with no witness
chip:
  label: KEY
  tone: accent
title: The privilege jump leaves no mark in your shell
aha: The shell is theatre. It shows you a prompt and echoes your commands, but it keeps no record of the microsecond your effective identity changed - because that change did not happen in the shell at all.
diagram:
  caption: the euid flips during exec; the shell narrates none of it
  shapes:
    - {type: box, x: 45, y: 118, w: 175, h: 64, label: 'your shell', sub: 'uid 10001', delay: 0}
    - {type: wire, from: {x: 220, y: 150}, to: {x: 300, y: 150}, delay: 0.4}
    - {type: box, x: 300, y: 108, w: 190, h: 84, label: 'SUID exec', sub: 'euid -> 0', tone: danger, delay: 0.7}
    - {type: label, x: 395, y: 214, text: 'no line printed, no log written', tone: danger, delay: 1.0}
---
When you run a `SUID`-root program, the kernel switches your process's effective
identity - its `euid` - to root for as long as that program runs. Your real
identity, your `uid`, never changes, so afterwards everything looks normal again.

Nothing in the shell announces this. `$ id` might still print your ordinary user,
your history just shows a command, and no log line is written unless some program
chose to write one. The privilege jump is real, and it is silent.
