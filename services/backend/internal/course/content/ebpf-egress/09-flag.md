---
kind: flag
ey: CAPTURE
eyt: The flag
tag: prove it
title: You read a closed connection off the kernel's record
aha: The connection is gone and the process may be too. The sensor kept it anyway - that is the difference between userspace evidence and kernel telemetry.
revealLabel: what this proves
---
You beaconed to the collector on the C2 port, and the sensor logged the outbound
connection at the socket - the flag rides that egress alert and starts with
`k8labs{`. Paste the whole value below to complete the lab. Notice what you did
not need: no open socket, no running process, no file - just the record the
kernel made of a connection the instant you opened it.
