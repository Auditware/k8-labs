---
kind: concept
ey: CONCEPT 1
eyt: Why the usual telemetry misses it
tag: technique
title: The evidence of a beacon lives in userspace, and dies there
aha: netstat, logs, shell history - all of it is written and read by userspace, and a foothold owns userspace. The one witness it does not own is the kernel.
diagram:
  caption: what an attacker can erase, and what they cannot
  shapes:
    - {type: box, x: 45, y: 100, w: 200, h: 52, label: 'history / logs / netstat', sub: 'userspace - erasable', tone: danger, delay: 0}
    - {type: box, x: 45, y: 168, w: 200, h: 52, label: 'the process', sub: 'exits, vanishes', tone: danger, delay: 0.4}
    - {type: wire, from: {x: 245, y: 160}, to: {x: 360, y: 145}, delay: 0.7}
    - {type: box, x: 360, y: 118, w: 175, h: 56, label: 'the connect()', sub: 'kernel saw it', tone: accent, delay: 0.9}
---
Think about how a beacon is usually caught, and why it usually is not. `netstat`
shows only connections that are still open - a beacon that fires and closes is
gone from it. Shell history, process lists, and application logs are all written
and read in userspace, and a foothold with any privilege can edit or wipe them.
Even the beaconing process itself can exit the moment it is done, leaving nothing
to inspect.

What none of that can undo is the connection. To send a single byte to the
collector, the process had to ask the kernel to open a socket - and that request
went through one kernel function every active TCP connection passes through.
Instrument that function, and the beacon is caught below the layer the attacker
controls.
