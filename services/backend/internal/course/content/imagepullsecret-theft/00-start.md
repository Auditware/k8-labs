---
kind: concept
ey: START
eyt: Image Pull Secret Theft
tag: what you'll learn
title: The most boring Secret in the namespace is a registry login
aha: A pull secret reads as plumbing - the thing that lets pods download images. It is actually a username and password for a registry, sitting in the cluster for anyone who can read Secrets.
revealLabel: why this matters
diagram:
  caption: routine-looking, but it is a credential
  shapes:
    - {type: box, x: 45, y: 116, w: 175, h: 66, label: 'you', sub: 'can read Secrets', delay: 0}
    - {type: wire, from: {x: 220, y: 149}, to: {x: 335, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 335, y: 116, w: 205, h: 66, label: 'regcred', sub: 'dockerconfigjson', tone: accent, delay: 0.7}
---
You have a foothold whose service account was handed a common, harmless-looking
over-grant: it can read Secrets in its namespace. No dramatic escalation, no
node access - just `get secrets`.

Sitting in that namespace is an `imagePullSecret`. It is the credential the
cluster uses to pull private images, and it looks like infrastructure. But strip
away one layer and it is a registry account - a username and password. Over the
next screens you will recognise it for what it is, decode it, and walk out with a
credential that reaches far outside this cluster.
