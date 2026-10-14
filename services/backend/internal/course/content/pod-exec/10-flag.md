---
kind: flag
ey: CAPTURE
eyt: The flag
tag: prove it
title: You borrowed another pod's identity to read what yours could not
aha: The quizzes show you followed the lateral move. The flag shows the borrowed token really worked.
revealLabel: what this proves
---
You exec'd into the CI pod and used its mounted token to read the `flag` Secret
in `production`, and it starts with `k8labs{`. Paste the whole value below to
complete the lab.

Still getting an empty result? `auth can-i --list` with no namespace (or `-A`)
checks `dev`, where you hold nothing - re-read **Your starting position** and run
every check `-n ops` before deciding the lab is broken.
