---
kind: concept
ey: START
eyt: Egress Beacon Detection
tag: what you'll learn
title: A process can hide. The connection it made cannot.
aha: Userspace forensics chase what a process left behind. eBPF network telemetry catches the one thing it cannot take back - the connection it already opened.
revealLabel: why this matters
diagram:
  caption: the beacon leaves, and the kernel writes it down
  shapes:
    - {type: box, x: 45, y: 116, w: 165, h: 66, label: 'your shell', sub: 'a foothold', delay: 0}
    - {type: wire, from: {x: 210, y: 149}, to: {x: 330, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 330, y: 116, w: 165, h: 66, label: 'collector', sub: 'C2 on :4444', tone: danger, delay: 0.7}
    - {type: box, x: 175, y: 210, w: 190, h: 44, label: 'eBPF logs the connect', tone: accent, delay: 1.0}
---
You have a foothold. The next thing an implant does is call home - open a
connection to a collector and beacon. So do it: connect out to the C2 on its
port. Nothing about the shell will look wrong afterwards.

But an eBPF sensor is watching the kernel, and it hooks the moment a socket
connects. The connection is written down at the socket - which process, to which
destination - the instant you make it. Over the next screens you will make the
beacon and then read it back off the sensor's feed, where the flag is waiting.
