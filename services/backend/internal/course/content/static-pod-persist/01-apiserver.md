---
kind: concept
ey: CONCEPT 1
eyt: The normal path
tag: foundation
title: Normally, every pod is checked on the way in
aha: The API server is the bouncer. Every request to run something is stopped, has its ID checked, and is frisked for anything against the rules.
diagram:
  caption: the front door checks every pod
  shapes:
    - {type: box, x: 45, y: 116, w: 150, h: 66, label: 'create pod', delay: 0}
    - {type: wire, from: {x: 195, y: 149}, to: {x: 285, y: 149}, delay: 0.4}
    - {type: box, x: 285, y: 108, w: 175, h: 84, label: 'API server', sub: 'RBAC + admission', tone: accent, delay: 0.7}
    - {type: wire, from: {x: 460, y: 149}, to: {x: 545, y: 149}, delay: 0.9}
    - {type: box, x: 545, y: 116, w: 75, h: 66, label: 'run', delay: 1.1}
---
When you ask Kubernetes to run a pod the usual way, the request goes to the `API
server`, which applies two gates. RBAC checks whether your identity may create
the pod at all. Then `admission` control - `PodSecurity` and any policy engines -
inspects the pod's spec and can reject dangerous shapes like a `privileged`
container or a host mount.

Every pod you have created so far went through both gates. They are the reason a
weak account cannot simply ask for a powerful pod. Which raises a question: what
if a pod could be started *without* going through the API server at all?
