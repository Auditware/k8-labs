---
kind: concept
ey: THE FIX
eyt: How to use it
tag: defence
title: Watch egress below userspace, and deny what is not needed
aha: Detection you cannot blind pairs with prevention you do not have to watch. The kernel feed tells you what a workload talks to; a default-deny egress policy makes sure it can talk to nothing else.
diagram:
  caption: see everything, allow only what is required
  shapes:
    - {type: box, x: 45, y: 116, w: 210, h: 66, label: 'eBPF egress feed', sub: 'every connect, un-eraseable', tone: accent, delay: 0}
    - {type: box, x: 300, y: 116, w: 245, h: 66, label: 'default-deny egress', sub: 'NetworkPolicy allow-list', tone: accent, delay: 0.6}
---
Two halves, one on each side of the beacon.

For detection: instrument egress in the kernel, not in userspace. A `tcp_connect`
feed is a full record of every outbound connection a workload makes, keyed to the
process and destination, that the workload cannot erase or hide by exiting.
Baseline what each pod legitimately connects to and alert on anything new - a
first-time destination is exactly the shape of a beacon.

For prevention: most pods talk to a tiny, fixed set of things. A default-deny
egress `NetworkPolicy` with an explicit allow-list means a compromised workload
can only reach what it was always meant to - so the beacon to a strange
destination does not just get logged, it does not connect at all. The sensor
tells you what happened; the policy makes sure most of it cannot.
