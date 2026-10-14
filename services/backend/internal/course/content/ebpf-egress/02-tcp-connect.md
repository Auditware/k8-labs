---
kind: concept
ey: CONCEPT 2
eyt: The hook
tag: technique
title: Every outbound TCP connection passes through tcp_connect
aha: This is not a syscall filter guessing at intent from arguments. It is the kernel's own connection routine, so it sees the real destination, resolved and about to be dialed.
diagram:
  caption: one kernel function on the path of every dial-out
  shapes:
    - {type: box, x: 45, y: 116, w: 150, h: 66, label: 'nc / curl', sub: 'connect()', delay: 0}
    - {type: wire, from: {x: 195, y: 149}, to: {x: 300, y: 149}, delay: 0.4}
    - {type: box, x: 300, y: 116, w: 165, h: 66, label: 'tcp_connect()', sub: 'kprobe here', tone: accent, delay: 0.6}
    - {type: wire, from: {x: 465, y: 149}, to: {x: 545, y: 149}, tone: danger, delay: 0.85}
    - {type: box, x: 520, y: 116, w: 95, h: 66, label: 'SYN', sub: 'out', tone: danger, delay: 1.05}
---
When a program opens an outbound TCP connection - `nc`, `curl`, a custom implant,
it does not matter which - the kernel runs `tcp_connect()` to start it. Tetragon
attaches a `kprobe` there and reads the function's socket argument, which already
holds the resolved destination address and port.

Two things make this stronger than a shell-level check. It fires on the
*initiating* side at connect time, so it does not need the handshake to finish -
a refused or filtered beacon is still logged. And because it reads the kernel's
own socket, there is no argument to spoof and no `netstat` to clear: the record
is made where the connection actually happens.
