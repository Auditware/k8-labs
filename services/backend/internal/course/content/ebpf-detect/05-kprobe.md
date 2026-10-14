---
kind: concept
ey: CONCEPT 4
eyt: Hooking the choke point
chip:
  label: KEY
  tone: accent
title: A kprobe attaches your eBPF onto a kernel function
aha: Recall the one function every privilege change flows through. A kprobe is how you nail your observer to that exact spot - and Tetragon is a tool that has already done it for you.
diagram:
  caption: hook the function, run eBPF, get an event with the culprit
  shapes:
    - {type: box, x: 40, y: 118, w: 175, h: 64, label: 'commit_creds()', sub: 'the kernel function', tone: danger, delay: 0}
    - {type: wire, from: {x: 215, y: 150}, to: {x: 290, y: 150}, tone: accent, delay: 0.4}
    - {type: box, x: 290, y: 116, w: 130, h: 68, label: 'kprobe', sub: 'eBPF hook', tone: accent, delay: 0.6}
    - {type: wire, from: {x: 420, y: 150}, to: {x: 490, y: 150}, delay: 0.9}
    - {type: box, x: 490, y: 118, w: 120, h: 64, label: 'Tetragon', sub: 'reports it', delay: 1.1}
---
A `kprobe` is the attachment point: "run this `eBPF` program whenever the kernel
calls this function." Point one at `commit_creds` and your program fires on every
credential the kernel installs, seeing which process caused it.

`Tetragon` is a security sensor that does exactly this. It hooks `commit_creds`
with a `kprobe` and turns each credential change into an event - the new
identity, and the program that triggered it.
