---
kind: concept
ey: CONCEPT 2
eyt: The privileged switch
chip:
  label: KEY IDEA
  tone: danger
title: privileged gives the container the node's full power
aha: It is the master switch. One boolean flips the container from "a locked-down process" to "root, with the run of the machine".
diagram:
  caption: one flag restores every dropped power
  shapes:
    - {type: box, x: 55, y: 116, w: 175, h: 66, label: 'privileged', sub: 'all capabilities', tone: danger, delay: 0}
    - {type: wire, from: {x: 230, y: 149}, to: {x: 345, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 345, y: 116, w: 210, h: 66, label: 'node-level power', sub: 'devices, /proc, mounts', tone: danger, delay: 0.7}
---
A container marked `privileged` has all the dropped `capability` bits restored and
its device restrictions lifted. It is close to running as root directly on the
node: it can access the node's block devices, load kernel modules, and - the part
that matters here - read into other processes.

On its own, `privileged` still has one of the two mechanisms working against you:
the pod's own `namespace` view means the node's processes are hidden. So the flag
is reachable only if the *other* switch - the one on the next screen - is also
flipped.

In a spec it is a single line, easy to miss in review:

```yaml
containers:
  - name: app
    securityContext:
      privileged: true   # all capabilities back, device restrictions lifted
```

Recognise it alongside its relatives - `allowPrivilegeEscalation: true`, a wide
`capabilities.add`, and the `hostPID` / `hostNetwork` fields from the sibling
lab. None are malformed; each is a normal field set to a node-equivalent value.
