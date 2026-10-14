---
kind: concept
ey: CONCEPT 3
eyt: Enumerating power
tag: technique
title: Don't poke endpoints - ask what you're allowed to do
aha: Attackers waste hours probing resources at random. The authorizer will just tell you your permissions if you ask it directly.
diagram:
  caption: ask the API to list your own permissions
  shapes:
    - {type: box, x: 40, y: 116, w: 240, h: 64, label: 'SelfSubjectRulesReview', sub: 'POST, per namespace', delay: 0}
    - {type: wire, from: {x: 280, y: 148}, to: {x: 380, y: 148}, tone: accent, delay: 0.4}
    - {type: box, x: 380, y: 116, w: 200, h: 64, label: 'your rules', sub: 'what you can do here', tone: accent, delay: 0.7}
---
Guessing which resources you can touch is slow and noisy. `RBAC` exposes the
answer directly: a `SelfSubjectRulesReview` returns the full list of what your
identity may do in a given namespace.

```
curl -s --cacert $CA -H "Authorization: Bearer $T" \
  -H 'Content-Type: application/json' \
  -X POST $API/apis/authorization.k8s.io/v1/selfsubjectrulesreviews \
  -d '{"apiVersion":"authorization.k8s.io/v1","kind":"SelfSubjectRulesReview","spec":{"namespace":"dev"}}'
```

Run it: `$ curl -s --cacert $CA -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -X POST $API/apis/authorization.k8s.io/v1/selfsubjectrulesreviews -d '{"apiVersion":"authorization.k8s.io/v1","kind":"SelfSubjectRulesReview","spec":{"namespace":"dev"}}'`.

Read `status.resourceRules`. In your own `dev` namespace it is thin. The
interesting question is what the same request returns for a namespace that is
not yours.
