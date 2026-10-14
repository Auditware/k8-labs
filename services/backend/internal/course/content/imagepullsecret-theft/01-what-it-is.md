---
kind: concept
ey: CONCEPT 1
eyt: What a pull secret holds
tag: technique
title: A dockerconfigjson Secret is a registry username and password
aha: The kubelet needs to log in to a private registry to pull images, so someone stored the login in the cluster. A login the kubelet can use is a login you can use.
diagram:
  caption: the same file docker login writes, stored as a Secret
  shapes:
    - {type: box, x: 45, y: 116, w: 175, h: 66, label: 'type:', sub: 'dockerconfigjson', tone: accent, delay: 0}
    - {type: wire, from: {x: 220, y: 149}, to: {x: 330, y: 149}, delay: 0.4}
    - {type: box, x: 330, y: 116, w: 210, h: 66, label: 'auths[registry]', sub: 'username + auth', tone: danger, delay: 0.7}
---
A Secret of type `kubernetes.io/dockerconfigjson` holds exactly what
`docker login` writes to disk: a small JSON document mapping each registry to the
credentials for it. The cluster keeps one so the kubelet can authenticate and
pull private images on a pod's behalf.

That is the catch. The credential has to be usable by the kubelet, which means it
is a real, working registry login - not a hash, not a token scoped to one pull.
Whoever reads the Secret gets the registry account itself. And registry access is
not a small thing: a registry you can write to is a registry whose images every
pod trusts.
