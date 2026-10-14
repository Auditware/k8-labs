---
kind: concept
ey: CONCEPT 3
eyt: Permissions
tag: the machinery
title: A Role is a list of allowed actions
aha: A Role is a job description pinned to the wall. On its own it grants nothing at all - nobody has been given the job yet.
diagram:
  caption: verbs x resources, and nothing else
  shapes:
    - {type: box, x: 190, y: 70, w: 260, h: 62, label: 'Role', sub: 'a named list of permissions', tone: accent, delay: 0}
    - {type: wire, from: {x: 260, y: 132}, to: {x: 200, y: 175}, delay: 0.4}
    - {type: wire, from: {x: 380, y: 132}, to: {x: 440, y: 175}, delay: 0.5}
    - {type: box, x: 110, y: 175, w: 180, h: 58, label: 'verbs', sub: 'get, list, create', delay: 0.7}
    - {type: box, x: 350, y: 175, w: 180, h: 58, label: 'resources', sub: 'pods, secrets', delay: 0.9}
---
Kubernetes decides who may do what with `RBAC`. Its smallest piece is a `Role`: a
named list pairing `verbs` (get, list, create) with `resources` (pods, secrets).

A Role by itself changes nothing. It describes permissions; it does not hand them
to anyone. Something else has to connect the list to an identity - and that
connector is where our lab goes wrong.
