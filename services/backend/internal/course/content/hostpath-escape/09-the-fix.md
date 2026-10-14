---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: The node is the boundary, and hostPath erases it
aha: RBAC gets all the attention because it is where access decisions feel like they live. But a single volume line can make every RBAC rule beside the point.
revealLabel: why it slips through
diagram:
  caption: forbid the mount at the door
  shapes:
    - {type: box, x: 55, y: 66, w: 250, h: 58, label: 'hostPath allowed', sub: 'before - node is exposed', tone: danger, delay: 0}
    - {type: wire, from: {x: 180, y: 124}, to: {x: 180, y: 169}, delay: 0.4}
    - {type: box, x: 55, y: 169, w: 250, h: 58, label: 'admission blocks it', sub: 'after - node is sealed', tone: accent, delay: 0.7}
    - {type: seal, cx: 470, cy: 148, r: 30, delay: 1.0}
    - {type: label, x: 470, y: 212, text: 'container stays a container', tone: accent, delay: 1.2}
---
The `workstation` pod was allowed a `hostPath` mount of the node's root. Enforce
`PodSecurity` at the `baseline` or `restricted` level (or an equivalent policy
engine) on every namespace that runs untrusted or general workloads - both block
`hostPath` outright, so the pod can never be admitted with that mount.

Two things worth carrying away. A container's isolation is only as strong as the
pod spec it was admitted with; dangerous fields - `hostPath`, `privileged`,
`hostPID`, `hostNetwork` - turn a container back into node access, so the control
that matters is admission, not RBAC. And when you assess a cluster, read the pod
specs, not just the roles: the escape is written plainly in the `volume` list.

The control is one label on the namespace - `baseline` already forbids
`hostPath`:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: workloads
  labels:
    pod-security.kubernetes.io/enforce: baseline
```

With that in place, the insecure pod from earlier is rejected at admission - it
never runs, so there is no node to reach.
