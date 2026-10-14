---
kind: flag
ey: CAPTURE
eyt: The flag
tag: prove it
title: You ran a pod the API server never got to see
aha: The quizzes show you followed the bypass. The flag shows the static pod really ran with the access you lacked.
revealLabel: what this proves
---
The manifest you dropped in the kubelet's `static pod` directory ran with the
node access you did not have and handed you the planted file, and it starts with
`k8labs{`. Paste the whole value below to complete the lab.

Nothing after a couple of minutes? The kubelet silently drops a manifest it
cannot parse, and the pod that runs is a mirror named `<name>-<node>`, not the
name you gave it - re-read **Your starting position** before deciding the lab is
broken.
