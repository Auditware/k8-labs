---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: The repair is one object kind, in one place
aha: This is why the mistake survives review so easily. Nothing looks alarming - the permissions are modest and the account is ordinary. Only the binding's kind gives it away.
revealLabel: why it slips through
diagram:
  caption: same role, same account, correct reach
  shapes:
    - {type: box, x: 60, y: 65, w: 240, h: 58, label: 'ClusterRoleBinding', sub: 'before - every namespace', tone: danger, delay: 0}
    - {type: wire, from: {x: 180, y: 123}, to: {x: 180, y: 168}, delay: 0.4}
    - {type: box, x: 60, y: 168, w: 240, h: 58, label: 'RoleBinding in dev', sub: 'after - dev only', tone: accent, delay: 0.7}
    - {type: seal, cx: 460, cy: 148, r: 32, delay: 1.0}
    - {type: label, x: 460, y: 215, text: 'production is out of reach', tone: accent, delay: 1.2}
---
The lab's `analyst` account was bound with a `ClusterRoleBinding`. Nobody needed
that - the intent was "let analyst read secrets in dev", and the fix is to bind
the very same `ClusterRole` with a `RoleBinding` inside `dev` instead.

Two things worth carrying away. Permissions are granted by bindings, so review
the binding's kind and not just the role's rules. And when you are handed an
identity, ask it what it can do somewhere you do not expect - that one question
is what turned a tidy-looking cluster into a cross-namespace read.
