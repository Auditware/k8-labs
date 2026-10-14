---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: Node access is cluster access - so guard the node, and thin the drawer
aha: Every escape lab ends here. Reaching the node feels like a local win, but the node is where the cluster's identities are kept, so a local win is a cluster-wide one.
revealLabel: why it matters
diagram:
  caption: seal the node, and mount fewer tokens
  shapes:
    - {type: box, x: 55, y: 66, w: 250, h: 58, label: 'node readable + tokens', sub: 'before - identities exposed', tone: danger, delay: 0}
    - {type: wire, from: {x: 180, y: 124}, to: {x: 180, y: 169}, delay: 0.4}
    - {type: box, x: 55, y: 169, w: 250, h: 58, label: 'sealed + fewer tokens', sub: 'after - little to steal', tone: accent, delay: 0.7}
    - {type: seal, cx: 470, cy: 148, r: 30, delay: 1.0}
    - {type: label, x: 470, y: 212, text: 'the drawer is nearly empty', tone: accent, delay: 1.2}
---
The harvest needed two things: a pod that could read the node, and powerful
`token` files sitting on that node to take. Close the first with admission -
`PodSecurity` blocking `hostPath` and `privileged` keeps pods off the node's
disk. Thin the second by setting `automountServiceAccountToken: false` on every
workload that does not call the API, and preferring short-lived, audience-bound
tokens, so fewer strong credentials are ever written to disk.

Two things worth carrying away. Treat "reached the node" as "reached the cluster"
in any assessment - it is the real prize behind every container escape. And least
privilege is not only about what an account can do; it is about not leaving the
account's key lying where a neighbour can pick it up.
