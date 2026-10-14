---
kind: concept
ey: CONCEPT 3
eyt: Code inside the kernel
chip:
  label: KEY
  tone: accent
title: eBPF runs your program inside the running kernel, safely
aha: It is the difference between standing outside a factory counting trucks and standing on the floor watching each machine. eBPF puts your observer where the events actually happen, without shutting the factory down to install it.
diagram:
  caption: a verified program, loaded live, watching from the inside
  shapes:
    - {type: box, x: 45, y: 118, w: 150, h: 62, label: 'your program', sub: 'small, restricted', delay: 0}
    - {type: wire, from: {x: 195, y: 149}, to: {x: 275, y: 149}, delay: 0.4}
    - {type: box, x: 275, y: 116, w: 130, h: 66, label: 'verifier', sub: 'proves it safe', tone: warn, delay: 0.6}
    - {type: wire, from: {x: 405, y: 149}, to: {x: 470, y: 149}, tone: accent, delay: 0.9}
    - {type: box, x: 470, y: 116, w: 140, h: 66, label: 'the kernel', sub: 'runs it live', tone: accent, delay: 1.1}
---
`eBPF` lets you load a small program into the running kernel and attach it to
events happening inside - process starts, syscalls, credential changes - with no
patch and no reboot. Your program sees exactly what the kernel sees.

The catch that makes it safe: before anything loads, a `verifier` proves the
program cannot loop forever or touch memory it should not. So you get a view from
inside the kernel without the usual risk of putting code there.
