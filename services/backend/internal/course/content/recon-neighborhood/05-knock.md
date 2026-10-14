---
kind: concept
ey: CONCEPT 4
eyt: Reaching it
chip: {label: the payoff, tone: danger}
title: Knock on the unlisted address with curl
aha: The service was never protected - only unlisted. Reachability was the whole security model, and reachability is exactly what you have.
diagram:
  caption: unlisted is not the same as unreachable
  shapes:
    - {type: box, x: 50, y: 116, w: 160, h: 64, label: 'you', sub: 'curl', delay: 0}
    - {type: wire, from: {x: 210, y: 148}, to: {x: 320, y: 148}, delay: 0.4}
    - {type: box, x: 320, y: 116, w: 190, h: 64, label: 'metrics service', sub: 'observability', tone: accent, delay: 0.6}
    - {type: wire, from: {x: 510, y: 148}, to: {x: 560, y: 148}, tone: danger, delay: 0.9}
    - {type: box, x: 555, y: 116, w: 65, h: 64, label: 'flag', tone: danger, delay: 1.1}
---
The endpoint is internal and unauthenticated - it trusts anything on the pod
network. So just ask it: `$ curl -s http://metrics.observability.svc.cluster.local`.

The response body starts with `k8labs{`. That is your flag: served by a `Service`
you were forbidden to `list`, found through the app's own config, resolved by
`DNS`, and reached over a network with no `NetworkPolicy` to stop you.
