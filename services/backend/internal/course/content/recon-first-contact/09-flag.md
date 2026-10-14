---
kind: flag
ey: CAPTURE
eyt: The flag
tag: prove it
title: The flag was in your pocket the whole time
aha: No exploit, no network, no kubectl - just the discipline to read what the pod already held.
revealLabel: what this proves
---
The database password leaked into your pod's `env` through a `secretKeyRef`, and
it starts with `k8labs{`. Read it back with `$ env` (or `$ printenv ORDERS_DB_PASSWORD`)
and paste the whole value below to complete the lab.
