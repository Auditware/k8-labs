---
kind: concept
ey: BRIEFING
eyt: Your foothold
chip: {label: recap, tone: accent}
title: Here is what you were handed
aha: A blocked API, a config file, a name, and an open network - four ordinary things that add up to a secret.
---
Retrace it. The API told you nothing - `list` on `services` was `403`. But the
app's mounted config named an upstream `Service`, `DNS` resolved that name to a
`ClusterIP` with no regard for your permissions, and the pod network carried
your `curl` straight to it.

The metrics endpoint answered with the flag. Capture it on the next screen.
