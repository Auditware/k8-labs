---
kind: concept
ey: CONCEPT 2
eyt: The side door
chip:
  label: KEY IDEA
  tone: danger
title: The kubelet runs pods straight from a directory on disk
aha: The kubelet does not only take orders from the API server. It also watches a folder, and treats every file in it as a pod to run - no questions asked.
diagram:
  caption: a manifest on disk becomes a running pod
  shapes:
    - {type: box, x: 55, y: 116, w: 185, h: 66, label: 'manifest on disk', sub: 'in the watched dir', tone: danger, delay: 0}
    - {type: wire, from: {x: 240, y: 149}, to: {x: 355, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 355, y: 116, w: 200, h: 66, label: 'kubelet runs it', sub: 'as a static pod', tone: danger, delay: 0.7}
---
The `kubelet` - the node agent that runs pods - watches a local directory called
its `staticPodPath` (on k3s, `/var/lib/rancher/k3s/agent/pod-manifests`). Any pod
manifest placed there is started as a `static pod`, run directly by the kubelet.

A static pod never goes to the `API server`. The kubelet reads the file and runs
the pod itself, then afterwards registers a read-only "mirror" copy in the API so
you can see it exists. But the decision to run it was made entirely on the node,
by a component whose whole job is to run what it is told.
