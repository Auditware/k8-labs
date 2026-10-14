---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: The task is a single connection. Make it, then turn to the Defender pane and read the record the kernel kept of it.
revealLabel: how to start
diagram:
  caption: beacon to the collector, read it off the sensor
  shapes:
    - {type: box, x: 45, y: 116, w: 160, h: 66, label: 'workstation', sub: 'your shell', delay: 0}
    - {type: wire, from: {x: 205, y: 149}, to: {x: 315, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 315, y: 116, w: 175, h: 66, label: 'collector.lab.svc', sub: ':4444', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 315, y: 182}, to: {x: 250, y: 220}, tone: accent, delay: 0.95}
    - {type: box, x: 150, y: 220, w: 200, h: 44, label: 'Defender: flag on the alert', tone: accent, delay: 1.15}
---
Your session drops you into a pod called `workstation`. Also in the cluster is a
`collector` - a listener on port `4444`, standing in for the box an implant would
beacon to, reachable at `collector.lab.svc`.

Call home: open a TCP connection to it. `$ nc -w2 collector.lab.svc 4444` is
enough - the connection itself is the event; you do not need to send anything,
and it is fine if it closes right away. Then open the `Defender` pane. The sensor
posts your beacon to `:4444`, keyed to your process and the destination, and the
flag rides that alert as its message. Read it there - not from any file, but from
the record the kernel kept of a connection you already closed.
