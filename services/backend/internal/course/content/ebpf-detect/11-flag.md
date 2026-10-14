---
kind: flag
ey: CAPTURE
eyt: The flag
tag: prove it
title: You escalated to root, and the sensor watched you do it
aha: The flag was inside the detection all along - reading the sensor's alert is what captures it. There was no file to cat; the kernel event is the reward.
revealLabel: what this proves
---
The eBPF sensor caught the exact moment your SUID exec installed root
credentials - and the flag rides that alert. In the `Defender` pane, find the
`commit_creds` event your escalation caused (euid 0 with your real uid still
unprivileged) and read the flag off that row. It starts with `k8labs{`. Paste
the whole value below to complete the lab.
