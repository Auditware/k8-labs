---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: The attacker's server is already standing. You are the one who arrives, points the cluster at it, and lets the next pod walk into the trap.
revealLabel: how to start
diagram:
  caption: register, re-roll, read the injected logs
  shapes:
    - {type: box, x: 45, y: 116, w: 165, h: 66, label: 'workstation', sub: 'registers webhook', tone: danger, delay: 0}
    - {type: wire, from: {x: 210, y: 149}, to: {x: 305, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 305, y: 116, w: 160, h: 66, label: 'orders pod', sub: '+ audit sidecar', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 465, y: 149}, to: {x: 525, y: 110}, tone: accent, delay: 0.95}
    - {type: box, x: 500, y: 60, w: 115, h: 50, label: 'flag', sub: 'in logs', tone: accent, delay: 1.15}
---
Your session drops you into `workstation` in the `lab` namespace. Confirm your
grant with `$ kubectl auth can-i --list` - you can create
`mutatingwebhookconfigurations`. A webhook server is already running as the
`injector` Service in the `attacker` namespace, and its CA is mounted for you at
`/etc/webhook-ca/ca.crt` (`$ base64 -w0 /etc/webhook-ca/ca.crt` gives the caBundle
value).

Write a `MutatingWebhookConfiguration` for pod `CREATE` whose `clientConfig`
points at that Service on path `/mutate` with that `caBundle`. Scope it - a
`namespaceSelector` for `production` and `failurePolicy` set to ignore - as the
last screen warned. Then re-roll the target: `$ kubectl delete pod -n production -l
app=orders` recreates it through your webhook, and the injected `audit` container
reads the flag. Read its logs to collect it.
