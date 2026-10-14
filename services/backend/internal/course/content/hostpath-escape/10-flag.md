---
kind: flag
ey: CAPTURE
eyt: The flag
tag: prove it
title: You read a file on the node from inside a container
aha: The quizzes show you spotted the mount. The flag shows the container boundary was really gone.
revealLabel: what this proves
---
Through the `hostPath` mount of the node's root you read the planted file under
the node's `/opt`, and it starts with `k8labs{`. Paste the whole value below to
complete the lab.
