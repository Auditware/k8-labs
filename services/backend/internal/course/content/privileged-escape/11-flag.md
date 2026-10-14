---
kind: flag
ey: CAPTURE
eyt: The flag
tag: prove it
title: You reached the node's filesystem through its own init
aha: You captured the flag and the sensor logged the dereference. Both halves of the lesson landed.
revealLabel: what this proves
---
Being privileged with `hostPID`, you read the node's root through `/proc/1/root`
and picked up the planted file under the node's `/opt`, and it starts with
`k8labs{`. Paste the whole value below to complete the lab - then check the
Defender pane for the open the sensor caught.
