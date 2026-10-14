---
kind: concept
ey: CONCEPT 2
eyt: The mutating kind
chip:
  label: KEY IDEA
  tone: danger
title: A mutating webhook does not just judge a pod - it edits it
aha: This is exactly how a service mesh slips its proxy into your pods. The same mechanism, pointed the wrong way, slips in an attacker's container.
diagram:
  caption: the webhook returns a patched pod
  shapes:
    - {type: box, x: 55, y: 116, w: 165, h: 66, label: 'pod as sent', sub: '1 container', delay: 0}
    - {type: wire, from: {x: 220, y: 149}, to: {x: 330, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 330, y: 116, w: 225, h: 66, label: 'pod as stored', sub: '+ your container', tone: danger, delay: 0.7}
---
There are two kinds of admission webhook. A validating one only says yes or no. A
`MutatingWebhookConfiguration` registers a webhook that can *rewrite* the object -
it returns a patch, and the API server applies it before storing the pod.

So a mutating webhook you control can add a container to every pod it sees. It is
the automated, cluster-wide version of editing a workload by hand - you do not
patch one Deployment, you register once and every future pod, across teams and
namespaces you never touched, comes out with your container inside.

Registering one is a single RBAC action: `create` on the
`mutatingwebhookconfigurations` resource. That one grant is what this lab turns on.
