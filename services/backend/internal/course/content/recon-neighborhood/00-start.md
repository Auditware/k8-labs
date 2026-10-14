---
kind: concept
ey: START
eyt: The Neighbourhood
tag: what you'll learn
title: The API said no. That is not the same as isolation.
aha: A 403 feels like a wall. It is only a wall on one road - and the cluster has several roads into the same house.
revealLabel: why this matters
diagram:
  caption: one road is blocked; the others are open
  shapes:
    - {type: box, x: 60, y: 116, w: 150, h: 64, label: 'you', sub: 'foothold', delay: 0}
    - {type: wire, from: {x: 210, y: 132}, to: {x: 320, y: 132}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 320, y: 100, w: 180, h: 44, label: 'API list', sub: '403 forbidden', tone: danger, delay: 0.6}
    - {type: wire, from: {x: 210, y: 164}, to: {x: 320, y: 164}, delay: 0.8}
    - {type: box, x: 320, y: 152, w: 180, h: 44, label: 'DNS + network', sub: 'wide open', tone: accent, delay: 1.0}
---
Try to list what is around you over the API:

```
API=https://kubernetes.default.svc
T=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
CA=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
curl -s --cacert $CA -H "Authorization: Bearer $T" $API/api/v1/namespaces/dev/services
```

It comes back `403`. Your account cannot list `services` over the API. It is
tempting to stop here and call the pod isolated - it is not. The API server is
only one way to learn about the cluster; this lab is about the others, which
never check your `RBAC` at all.
