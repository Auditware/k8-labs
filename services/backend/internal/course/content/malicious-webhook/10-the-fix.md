---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: Webhook config is admission itself - guard it like cluster-admin
aha: It reads like plumbing for Istio or a policy engine. But it is a seat at the table where every object is decided, and a seat there is a seat everywhere.
revealLabel: why it matters
diagram:
  caption: lock the registry, watch for new webhooks
  shapes:
    - {type: box, x: 55, y: 66, w: 250, h: 58, label: 'anyone registers webhooks', sub: 'before - admission is open', tone: danger, delay: 0}
    - {type: wire, from: {x: 180, y: 124}, to: {x: 180, y: 169}, delay: 0.4}
    - {type: box, x: 55, y: 169, w: 250, h: 58, label: 'locked + audited', sub: 'after - admission is trusted', tone: accent, delay: 0.7}
    - {type: seal, cx: 470, cy: 148, r: 30, delay: 1.0}
    - {type: label, x: 470, y: 212, text: 'no uninvited reviewers', tone: accent, delay: 1.2}
---
The foothold could `create` a `MutatingWebhookConfiguration`, and that alone let
it rewrite every pod admitted in a namespace. Grant `create`/`update` on
`*webhookconfigurations` to almost nobody - the handful of controllers that
genuinely install webhooks - and alert on every new or changed configuration, the
same way you would on a new cluster-admin binding.

Two things worth carrying away. Admission control is a trust boundary that sits
above RBAC on individual objects: whoever can register a webhook can tamper with
everything RBAC was carefully protecting. And in an assessment, enumerate the
webhook configurations early - a rogue one, injecting a container into every pod,
is the textbook persistence backdoor, and it hides in plain sight among the
legitimate meshes and policy engines that use the exact same mechanism.
