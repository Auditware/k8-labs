---
kind: concept
ey: CONCEPT 1
eyt: A pod's identity
tag: foundation
title: A pod runs as whatever account its spec names
aha: The pod does not earn its identity. It is simply assigned one in a single line of YAML - and nothing checks whether you were entitled to assign it.
diagram:
  caption: one field decides the pod's identity
  shapes:
    - {type: box, x: 60, y: 108, w: 215, h: 90, label: 'pod spec', sub: 'serviceAccountName: X', delay: 0}
    - {type: wire, from: {x: 275, y: 153}, to: {x: 385, y: 153}, tone: accent, delay: 0.5}
    - {type: box, x: 385, y: 120, w: 185, h: 66, label: 'account X token', sub: 'mounted in the pod', tone: accent, delay: 0.8}
---
Every pod runs as a `ServiceAccount`, chosen by one field in its spec:
`serviceAccountName`. Whatever account that field names, Kubernetes mounts *that*
account's `token` into the pod, and processes inside use it to call the API.

The field is just a name. When you create a pod, you fill it in - and there is
no separate permission that asks "are you allowed to run as this account". If the
account exists in the namespace, you can name it.
