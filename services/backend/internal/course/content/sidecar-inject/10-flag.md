---
kind: flag
ey: CAPTURE
eyt: The flag
tag: prove it
title: You added a container that read a token that was never yours
aha: The quizzes show you followed the injection. The flag shows the sidecar really borrowed the token.
revealLabel: what this proves
---
Your patched `sidecar` came up inside the payments pods, borrowed their token,
and logged the `production` flag, and it starts with `k8labs{`. Paste the whole
value below to complete the lab.
