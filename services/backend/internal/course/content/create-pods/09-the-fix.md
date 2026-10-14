---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: create pods is run-as-any-account-here
aha: It survives review because it reads like plumbing for a deploy pipeline. But the pod spec is a form the attacker fills in, and one of its fields is "whose credentials do I boot with".
revealLabel: why it slips through
diagram:
  caption: scope the verb, or gate the account choice
  shapes:
    - {type: box, x: 55, y: 66, w: 250, h: 58, label: 'create pods, unscoped', sub: 'before - run as any account', tone: danger, delay: 0}
    - {type: wire, from: {x: 180, y: 124}, to: {x: 180, y: 169}, delay: 0.4}
    - {type: box, x: 55, y: 169, w: 250, h: 58, label: 'scoped or admission-gated', sub: 'after - only own account', tone: accent, delay: 0.7}
    - {type: seal, cx: 470, cy: 148, r: 30, delay: 1.0}
    - {type: label, x: 470, y: 212, text: 'builder cannot be worn', tone: accent, delay: 1.2}
---
The foothold held `create pods` on a namespace that also housed a powerful
account. Grant pod creation narrowly - to the namespaces and roles that truly
need it - and add an admission control (a policy engine, or a restricted
PodSecurity path) that limits which `serviceAccountName` a creator may set.

Two things worth carrying away. Treat any workload-creating verb - pods,
deployments, jobs, cronjobs - as "run code as the accounts in this namespace",
because that is what it is. And remember the account choice is unguarded by RBAC
alone; if it matters, it must be enforced at admission, not assumed.
