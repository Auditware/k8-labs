---
kind: concept
ey: CONCEPT 4
eyt: The injected container needs an image
chip:
  label: WATCH OUT
  tone: warn
title: Inject an image the cluster already has
aha: This cluster cannot reach the internet. Name an outside image and your sidecar never starts - the whole roll stalls waiting for a download that will not arrive.
diagram:
  caption: no registry - inject a local image
  shapes:
    - {type: box, x: 55, y: 116, w: 175, h: 66, label: 'sidecar', sub: 'image: nginx', tone: danger, delay: 0}
    - {type: wire, from: {x: 230, y: 149}, to: {x: 340, y: 149}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 340, y: 116, w: 205, h: 66, label: 'internet registry', sub: 'unreachable - hangs', tone: danger, delay: 0.7}
---
One detail decides whether your sidecar ever runs. This lab cluster is airgapped -
it pulls no images. A container that asks for a public image sits in
`ImagePullBackOff`, and a rolling Deployment can stall on it.

Use an image already baked into the cluster: `k8labs-foothold:1.31`, the same
shell-with-kubectl image your foothold runs. Set `imagePullPolicy` to `Never` so
the container uses the local copy. Real clusters differ, but the habit holds -
inject something you know is present.
