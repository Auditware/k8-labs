---
kind: concept
ey: CONCEPT 4
eyt: The escape hatch
chip:
  label: THE FLAW
  tone: danger
title: One verb lets you grant a role you do not hold
aha: The safeguard has a documented exception, meant for controllers. Handed to an attacker, it is a signed blank cheque for one specific role.
diagram:
  caption: bind waves the grant past the check
  shapes:
    - {type: box, x: 45, y: 116, w: 150, h: 66, label: 'you + bind', sub: 'on secret-admin', tone: danger, delay: 0}
    - {type: wire, from: {x: 195, y: 149}, to: {x: 300, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 300, y: 116, w: 175, h: 66, label: 'check skipped', sub: 'for that role', tone: warn, delay: 0.7}
    - {type: wire, from: {x: 475, y: 149}, to: {x: 560, y: 149}, tone: danger, delay: 0.9}
    - {type: seal, cx: 585, cy: 149, r: 18, broken: true, delay: 1.0}
---
The safeguard has a deliberate exception: the `bind` verb. If you hold `bind` on
a particular role, the API server lets you grant *that* role to anyone, even
though you do not hold its permissions yourself. A sibling verb, `escalate`, does
the same for editing a role's contents.

These exist so automation - a controller that provisions access - can hand out
roles without being granted everything under the sun. But `bind` on a powerful
role like `secret-admin` (a `ClusterRole` in this lab), given to an account that
can also create bindings, is the exact combination that defeats the check. The
safeguard is still running; you were simply handed the key that opens it.

This is what that combination looks like in a manifest - a single `Role`, nothing
malformed, that hands one account both halves. It lives in `production` because
that is where the self-grant is written, and it names `bind` on `clusterroles`
because `secret-admin` is cluster-scoped - the resource in the `bind` rule must
match the *kind* of role you are binding, or the API server denies it. The fixed
tab shows the same role scoped to what it actually needs:

```yaml role-access-delegator.yaml
kind: Role
metadata:
  name: access-delegator          # reads like "manages who gets access"
  namespace: production           # where the binding gets created
rules:
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["rolebindings"]
    verbs: ["create"]             # ! can mint a binding for any subject, you included
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["clusterroles"]   # ! secret-admin is a ClusterRole, so bind clusterroles
    resourceNames: ["secret-admin"]
    verbs: ["bind"]               # ! and grant secret-admin without ever holding it
```

```yaml role-scoped.yaml
kind: Role
metadata:
  name: access-delegator
  namespace: production
rules:
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["app-config"]   # + scoped to the one object it reads
    verbs: ["get", "list"]          # + no create on rolebindings, no bind verb
```

Neither rule is wrong on its own, and `bind` even reads as the *narrower*, safer
verb. Together they are the self-grant. The pattern to flag in a review is one
subject holding `create` on `rolebindings` alongside `bind` on any role worth
having.
