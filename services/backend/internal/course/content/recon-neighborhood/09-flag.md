---
kind: flag
ey: CAPTURE
eyt: The flag
tag: prove it
title: The flag was answering on a port nobody advertised
aha: No kubectl, no RBAC bypass - just a name from a config file and a network that never asked who you were.
revealLabel: what this proves
---
You read the app's config, resolved `metrics.observability.svc.cluster.local`
with `DNS`, and curled the internal `Service` the API would not let you `list`.
The response starts with `k8labs{` - paste the whole value below to complete the
lab.
