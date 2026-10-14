---
kind: concept
ey: CONCEPT 1
eyt: Where tokens live
tag: foundation
title: The kubelet writes every pod's token to the node's disk
aha: A service-account token feels like a secret held safely inside a pod. It is really a file on the node, placed there by the component that runs the pods.
diagram:
  caption: one node, every pod's token on its disk
  shapes:
    - {type: box, x: 55, y: 116, w: 165, h: 66, label: 'kubelet', sub: 'runs the pods', delay: 0}
    - {type: wire, from: {x: 220, y: 149}, to: {x: 330, y: 149}, delay: 0.4}
    - {type: box, x: 330, y: 116, w: 225, h: 66, label: 'node disk', sub: 'a token per pod', tone: danger, delay: 0.7}
---
The `kubelet` is the agent on each node that actually runs pods. When a pod uses a
`ServiceAccount`, the kubelet obtains that account's `token` and mounts it into
the pod - and to do that, it first writes the token to the node's own disk, under
a path like `/var/lib/kubelet/pods/<pod-id>/volumes/.../token`.

So every token belonging to every pod on a node exists as a plain file on that
node. Inside a pod you only see your own. But from the node's filesystem, they
are all just files sitting side by side.
