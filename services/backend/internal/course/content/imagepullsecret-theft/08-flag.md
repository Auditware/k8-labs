---
kind: flag
ey: CAPTURE
eyt: The flag
tag: prove it
title: You walked off with a registry account
aha: You did not escalate or break out. You read one over-shared Secret and recognised a credential the cluster left in plain sight.
revealLabel: what this proves
---
You read the `regcred` pull secret, peeled its two base64 layers, and recovered
the registry password - it starts with `k8labs{`. Paste the whole value below to
complete the lab. What you are holding is not a lab token; it is the shape of a
real registry account, the kind that turns `get secrets` on one namespace into a
foothold in everything that namespace pulls.
