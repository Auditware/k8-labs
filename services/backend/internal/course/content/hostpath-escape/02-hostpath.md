---
kind: concept
ey: CONCEPT 2
eyt: The dangerous mount
chip:
  label: KEY IDEA
  tone: danger
title: A hostPath volume punches a hole to the node's filesystem
aha: It is a door cut into the container wall, opening onto the node. Mount the node's root, and the door is the whole wall.
diagram:
  caption: hostPath maps a node path into the pod
  shapes:
    - {type: box, x: 55, y: 116, w: 175, h: 66, label: 'your pod', sub: '/host', delay: 0}
    - {type: wire, from: {x: 230, y: 149}, to: {x: 345, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 345, y: 116, w: 210, h: 66, label: 'node path /', sub: 'the whole disk', tone: danger, delay: 0.7}
---
A `volume` gives a pod storage. Most kinds are harmless - a scratch disk, a
mounted Secret. One kind is not: a `hostPath` volume mounts a path from the
*node's* filesystem directly into the pod. Mount the node's `/`, and your pod can
read the node's entire disk at whatever path you mounted it - say `/host`.

At that point the container's isolation is decorative. Everything the node can
see, your pod can see: other pods' mounted secrets, the kubelet's kubeconfig, the
container runtime's socket. A `hostPath` of `/` is not a storage feature; it is a
window onto the machine.

Concretely, the insecure part is these few lines in a pod spec - a `hostPath` of
`/` mounted into the container:

```yaml
volumes:
  - name: host
    hostPath:
      path: /              # the node's root filesystem
containers:
  - name: app
    volumeMounts:
      - name: host
        mountPath: /host   # /host in the pod now IS the node's /
```

Nothing here is malformed. It is a valid spec a cluster will happily run - the
danger is an ordinary field set to an over-broad value. That is the shape to
recognise in a real manifest review: not a typo, a `path` that reaches too far.
