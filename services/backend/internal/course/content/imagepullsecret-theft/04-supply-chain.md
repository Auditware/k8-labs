---
kind: concept
ey: CONCEPT 3
eyt: Why it is worth more than a Secret
tag: technique
title: Registry write access poisons every pod that pulls
aha: Reading one Secret gets you a key that reaches outside the cluster - and back into it, through every image the cluster will ever pull from that registry.
diagram:
  caption: one stolen login, every future pull compromised
  shapes:
    - {type: box, x: 40, y: 116, w: 150, h: 66, label: 'registry login', sub: 'stolen', tone: danger, delay: 0}
    - {type: wire, from: {x: 190, y: 149}, to: {x: 250, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 250, y: 116, w: 165, h: 66, label: 'push :latest', sub: 'poisoned image', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 415, y: 149}, to: {x: 470, y: 149}, tone: danger, delay: 0.95}
    - {type: box, x: 470, y: 116, w: 140, h: 66, label: 'every pod', sub: 'runs it', tone: danger, delay: 1.15}
---
A stolen database password lets you read a database. A stolen *registry* password
is bigger, because of what a registry is: the source of truth for the code the
whole cluster runs. If the credential can push, an attacker can overwrite an image
tag with a poisoned build, and the next time any workload pulls that tag - a
scale-up, a node reboot, a rollout - it runs the attacker's code, with that
workload's identity, wherever it is scheduled.

That is a supply-chain compromise, and it started with `get secrets` on a
namespace. This is why an imagePullSecret deserves the same paranoia as a
cluster-admin credential, not the shrug it usually gets as "just pull config".
