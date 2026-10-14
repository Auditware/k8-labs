---
kind: concept
ey: CONCEPT 3
eyt: What that lets you do
tag: the machinery
title: A static pod can hold powers the API server would refuse
aha: The manifest is a wish list, and the kubelet grants it in full. Ask for the node itself, and there is no gate left to say no.
diagram:
  caption: request what admission would have blocked
  shapes:
    - {type: box, x: 55, y: 116, w: 195, h: 66, label: 'static manifest', sub: 'privileged, hostPath /', tone: danger, delay: 0}
    - {type: wire, from: {x: 250, y: 149}, to: {x: 360, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 360, y: 116, w: 195, h: 66, label: 'runs as asked', sub: 'node access granted', tone: danger, delay: 0.7}
---
Because no `admission` gate sees a static pod, its manifest can ask for anything.
A `privileged` container, a `hostPath` mount of the node's `/` - the exact
settings `PodSecurity` exists to block - are all honoured, because the kubelet
does not enforce `PodSecurity` on the pods it runs itself.

So even if your account could never create a privileged pod through the `API
server`, you can still *run* one: write its manifest into the kubelet's directory.
The static pod becomes your tool - it can mount the whole node and read what your
foothold cannot, then hand the result back to you.
