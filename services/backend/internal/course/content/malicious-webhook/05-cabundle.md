---
kind: concept
ey: CONCEPT 4
eyt: Wiring it up
tag: the machinery
title: The API server only calls a webhook it can trust over TLS
aha: The one hurdle is honesty about identity - the API server insists on a verified HTTPS connection, so your config has to vouch for the server it points at.
diagram:
  caption: the config carries the server's CA
  shapes:
    - {type: box, x: 55, y: 116, w: 175, h: 66, label: 'webhook config', sub: 'caBundle + URL', tone: danger, delay: 0}
    - {type: wire, from: {x: 230, y: 149}, to: {x: 345, y: 149}, tone: accent, delay: 0.4}
    - {type: box, x: 345, y: 116, w: 210, h: 66, label: 'injector server', sub: 'over verified TLS', tone: accent, delay: 0.7}
---
A webhook endpoint must be HTTPS, and the `API server` verifies the server's
certificate against a `caBundle` you put in the config. So a
`MutatingWebhookConfiguration` names three things: which operations to intercept
(pod creates), where to send them (a Service or URL), and the `caBundle` that
trusts the server's `TLS` certificate.

In this lab the hard part is done for you: a webhook server is already running,
and its CA is handed to you as a file. Your job is to write the config that wires
the API server to it - the same config a real attacker would drop after standing
up their own server.
