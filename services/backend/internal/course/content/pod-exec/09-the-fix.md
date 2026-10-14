---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: Read pods/exec as "become any pod here"
aha: It hides in review because it sounds operational, like log access. But exec is identity access - the right to wear every badge in the room.
revealLabel: why it slips through
diagram:
  caption: scope exec, and do not mount idle tokens
  shapes:
    - {type: box, x: 55, y: 66, w: 250, h: 58, label: 'exec on the namespace', sub: 'before - every pod reachable', tone: danger, delay: 0}
    - {type: wire, from: {x: 180, y: 124}, to: {x: 180, y: 169}, delay: 0.4}
    - {type: box, x: 55, y: 169, w: 250, h: 58, label: 'exec on named pods', sub: 'after - only what is needed', tone: accent, delay: 0.7}
    - {type: seal, cx: 470, cy: 148, r: 30, delay: 1.0}
    - {type: label, x: 470, y: 212, text: 'ci stays reachable to no one', tone: accent, delay: 1.2}
---
The foothold was granted `pods/exec` across an entire namespace. Scope it to the
specific pods a role genuinely needs to debug (`resourceNames`), or do not grant
it at all - and set `automountServiceAccountToken: false` on pods that never call
the API, so an attacker who does get inside finds no `token` to steal.

Two things worth carrying away. `exec` is not a read verb; it is the authority to
become another workload, so weigh it like you would cluster-admin on that
namespace. And defence in depth matters here: even a reachable pod hands over
nothing if it carries no mounted identity to take.
