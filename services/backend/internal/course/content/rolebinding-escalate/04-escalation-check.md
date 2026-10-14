---
kind: concept
ey: CONCEPT 3
eyt: The safeguard
chip:
  label: KEY IDEA
  tone: accent
title: You may only grant permissions you already hold
aha: It is the rule that stops a junior admin from writing themselves a promotion. You can only pass along authority you already carry.
diagram:
  caption: the grant must be a subset of what you hold
  shapes:
    - {type: box, x: 55, y: 116, w: 185, h: 66, label: 'grant you attempt', sub: 'more than you hold', tone: danger, delay: 0}
    - {type: wire, from: {x: 240, y: 149}, to: {x: 355, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 355, y: 116, w: 200, h: 66, label: 'API server', sub: 'refuses the binding', tone: warn, delay: 0.7}
    - {type: label, x: 300, y: 210, text: 'not currently held', tone: danger, delay: 1.0}
---
Kubernetes calls this privilege-escalation prevention. When you create a
`RoleBinding`, the API server checks that you personally already hold every
permission the referenced role would grant. If the role reaches further than you
do, the request is refused with a telling message - that you are attempting to
grant permissions *not currently held*.

This is why `create rolebindings` is not automatically fatal. The safeguard turns
"I can grant any role" into "I can grant only roles I already have" - which,
normally, is no escalation at all. Normally.
