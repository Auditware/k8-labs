---
kind: concept
ey: START
eyt: eBPF Detection
tag: what you'll learn
title: You will become root, and watch something watch you do it.
aha: The escalation is the easy part - you have done it before. The point of this lab is what it looks like from below, where the kernel sees a privilege jump the shell never mentions.
revealLabel: why this matters
diagram:
  caption: you escalate; a sensor in the kernel sees it happen
  shapes:
    - {type: box, x: 45, y: 120, w: 160, h: 60, label: 'you', sub: 'unprivileged', delay: 0}
    - {type: wire, from: {x: 205, y: 150}, to: {x: 315, y: 150}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 315, y: 120, w: 150, h: 60, label: 'euid 0', sub: 'root, in an instant', tone: danger, delay: 0.7}
    - {type: box, x: 300, y: 55, w: 200, h: 46, label: 'eBPF sensor', sub: 'watching the kernel', tone: accent, delay: 1.0}
    - {type: wire, from: {x: 400, y: 101}, to: {x: 400, y: 120}, tone: accent, delay: 1.2}
---
You have an unprivileged shell. There is a way to become root on this machine,
and over the next few screens you will take it.

But that is not really the lesson. The lesson is that the moment you escalate,
something below your shell notices - not a log, not your command history, but the
kernel itself. This lab is about learning to see the privilege jump from there.
