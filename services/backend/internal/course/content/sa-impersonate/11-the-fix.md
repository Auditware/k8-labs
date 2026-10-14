---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: Treat impersonate as the most dangerous verb you can grant
aha: It slips through review because it reads as plumbing, not power. But a verb that lets an account choose who it is has quietly granted every permission the cluster can express.
revealLabel: why it slips through
diagram:
  caption: scope it, or do not grant it
  shapes:
    - {type: box, x: 55, y: 66, w: 250, h: 58, label: 'impersonate on groups', sub: 'before - reaches system:masters', tone: danger, delay: 0}
    - {type: wire, from: {x: 180, y: 124}, to: {x: 180, y: 169}, delay: 0.4}
    - {type: box, x: 55, y: 169, w: 250, h: 58, label: 'no impersonate', sub: 'after - one named user, or none', tone: accent, delay: 0.7}
    - {type: seal, cx: 470, cy: 148, r: 30, delay: 1.0}
    - {type: label, x: 470, y: 212, text: 'system:masters unreachable', tone: accent, delay: 1.2}
---
The account was granted `impersonate` on `groups` with no restriction. Almost
nothing legitimately needs that. The fix is to remove it - or, where a component
genuinely must impersonate, to grant it only for a specific named user with
`resourceNames`, never for `groups` at large.

Two things worth carrying away. `impersonate` is not a convenience verb; it is
the authority to become anyone, so it belongs in the same review tier as
cluster-admin itself. And a superuser group like `system:masters` cannot be
walled off with a Role or a namespace - its cluster-admin binding is protected
and cluster-wide, so the only defence is never letting an account name it. When you are handed an identity, the first question is not what
it can read, but whose identity it can put on.
