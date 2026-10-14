---
kind: flag
ey: CAPTURE
eyt: The flag
tag: prove it
title: You backdoored every pod the cluster admitted
aha: The quizzes show you followed the admission abuse. The flag shows your webhook really rewrote a live pod.
revealLabel: what this proves
---
Your `mutating webhook` injected a container into the re-rolled production pod,
and that container captured the flag, which starts with `k8labs{`. Paste the
whole value below to complete the lab.
