---
kind: concept
ey: CONCEPT 4
eyt: A pod needs an image
chip:
  label: WATCH OUT
  tone: warn
title: The pod must use an image the cluster already has
aha: This cluster is sealed off - it cannot fetch anything from the internet. Ask it for an outside image and the pod just sits there, waiting for a download that will never come.
diagram:
  caption: no registry to pull from - use what is local
  shapes:
    - {type: box, x: 55, y: 116, w: 175, h: 66, label: 'your pod', sub: 'image: nginx', tone: danger, delay: 0}
    - {type: wire, from: {x: 230, y: 149}, to: {x: 340, y: 149}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 340, y: 116, w: 205, h: 66, label: 'internet registry', sub: 'unreachable - hangs', tone: danger, delay: 0.7}
---
One practical detail decides whether your pod ever runs. This lab cluster is
airgapped - it pulls no images from any registry. A pod asking for `nginx` or any
public image will sit in `ImagePullBackOff` forever.

So use an image already baked into the cluster: `k8labs-foothold:1.31` - the same
shell-with-kubectl image your foothold runs. Set `imagePullPolicy` to `Never` so
the pod uses the local copy and never tries to pull. Real clusters differ, but
the habit is sound - know what is actually available before you launch.
