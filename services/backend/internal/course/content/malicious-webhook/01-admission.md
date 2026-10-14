---
kind: concept
ey: CONCEPT 1
eyt: The admission gate
tag: foundation
title: Every object is inspected as it enters, and webhooks are that inspection
aha: The API server does not just store what you send. It shows each object to a set of reviewers first - and lets you register as one of them.
diagram:
  caption: admission calls out to registered webhooks
  shapes:
    - {type: box, x: 45, y: 116, w: 150, h: 66, label: 'create pod', delay: 0}
    - {type: wire, from: {x: 195, y: 149}, to: {x: 285, y: 149}, delay: 0.4}
    - {type: box, x: 285, y: 116, w: 160, h: 66, label: 'admission', sub: 'calls webhooks', tone: accent, delay: 0.7}
    - {type: wire, from: {x: 445, y: 149}, to: {x: 545, y: 149}, delay: 0.9}
    - {type: box, x: 545, y: 116, w: 75, h: 66, label: 'store', delay: 1.1}
---
When an object is created, the `API server` runs it through `admission` before
saving it. Part of admission is configurable: the API server will call out to
`webhook` servers you have registered, sending them the object and using their
answer.

This is a normal, heavily used feature - service meshes, cert-manager, and policy
engines are all admission webhooks. The API server trusts a registered webhook to
inspect, and sometimes change, everything that passes through. Which makes "who
may register a webhook" a very load-bearing question.
