---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: Treat create-rolebindings plus bind as latent admin
aha: Each grant looks reasonable alone - one is "manages access", the other is "for a controller". Together they are a quiet path to any role they can name.
revealLabel: why it slips through
diagram:
  caption: neither grant is safe next to the other
  shapes:
    - {type: box, x: 55, y: 66, w: 250, h: 58, label: 'create bindings + bind', sub: 'before - self-grant path', tone: danger, delay: 0}
    - {type: wire, from: {x: 180, y: 124}, to: {x: 180, y: 169}, delay: 0.4}
    - {type: box, x: 55, y: 169, w: 250, h: 58, label: 'split or drop bind', sub: 'after - check holds', tone: accent, delay: 0.7}
    - {type: seal, cx: 470, cy: 148, r: 30, delay: 1.0}
    - {type: label, x: 470, y: 212, text: 'no role to self-grant', tone: accent, delay: 1.2}
---
The account was given both the ability to create `RoleBinding` objects and the
`bind` verb on a powerful role. Keep those apart: grant `bind`/`escalate` only to
the controllers that genuinely need them, never to a general workload, and audit
every use - a binding whose subject is the same identity that created it is a red
flag.

Two things worth carrying away. The escalation-prevention check is real and
useful, so an attack here is really an attack on its two documented exceptions,
`bind` and `escalate` - watch those verbs like you watch cluster-admin. And any
account that can both author bindings and bind a strong role should be read as
already holding that role, because in one command it does.
