---
kind: concept
ey: START
eyt: Malicious Admission Webhook
tag: what you'll learn
title: One configuration object, and every future pod is yours.
aha: You are not attacking a pod. You are attacking the gate every pod walks through - and telling it to tamper with each one on the way in.
revealLabel: why this matters
diagram:
  caption: rewrite the gate, and it rewrites everyone
  shapes:
    - {type: box, x: 55, y: 115, w: 180, h: 66, label: 'your webhook', sub: 'one config object', tone: danger, delay: 0}
    - {type: wire, from: {x: 235, y: 148}, to: {x: 350, y: 148}, tone: danger, delay: 0.4}
    - {type: box, x: 350, y: 115, w: 210, h: 66, label: 'every new pod', sub: 'gets your container', tone: danger, delay: 0.7}
---
You have a shell that can create one particular kind of object - an admission
webhook - and read no secret directly. This is the persistence capstone: instead
of stealing one identity, you rewire the cluster so that every pod created from
now on quietly carries your code.

Over the next few screens we will build up how the admission pipeline works, and
why the ability to register a webhook is one of the most powerful grants a cluster
can hand out.
