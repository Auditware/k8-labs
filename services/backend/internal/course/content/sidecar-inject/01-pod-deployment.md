---
kind: concept
ey: CONCEPT 1
eyt: How apps run
tag: foundation
title: A Deployment is a template that stamps out pods
aha: You do not edit the running pods directly. You edit the mould they are poured from, and the cluster recasts them for you.
diagram:
  caption: the template is the thing you can change
  shapes:
    - {type: box, x: 55, y: 116, w: 175, h: 66, label: 'Deployment', sub: 'a pod template', delay: 0}
    - {type: wire, from: {x: 230, y: 149}, to: {x: 340, y: 149}, delay: 0.4}
    - {type: box, x: 340, y: 116, w: 220, h: 66, label: 'pods it creates', sub: 'containers inside', tone: accent, delay: 0.7}
---
Most workloads are run by a `Deployment` - an object holding a `Pod` template.
The cluster reads that template and creates pods to match it, each running one or
more `container` processes. When you change the template, the Deployment rolls
the pods: it replaces the old ones with new pods built to the new spec.

So the pods you see are downstream of the template. You do not have to touch a
running pod to change what runs in it - you change the `Deployment`, and the next
generation of pods carries your change.
