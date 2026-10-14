---
kind: concept
ey: BRIEFING
eyt: Your foothold
chip: {label: recap, tone: accent}
title: Here is what you were handed
aha: Recon is not one clever trick - it is refusing to skip the boring inventory. The boring inventory held the whole prize.
diagram:
  caption: three findings, none of which needed the network
  shapes:
    - {type: box, x: 60, y: 116, w: 160, h: 64, label: 'namespace', sub: 'where you are', delay: 0}
    - {type: box, x: 240, y: 116, w: 150, h: 64, label: 'token', sub: 'who you are', delay: 0.3}
    - {type: box, x: 410, y: 116, w: 170, h: 64, label: 'leaked secret', sub: 'the objective', tone: danger, delay: 0.6}
---
Take stock. Without leaving the pod you have: your `namespace` and the cluster
domain (from the `namespace` file and `resolv.conf`), your `ServiceAccount`
`token` on disk, and a database credential leaked into `env` through a
`secretKeyRef`.

That last one is the objective. Read it back off your own environment and
capture it on the next screen.
