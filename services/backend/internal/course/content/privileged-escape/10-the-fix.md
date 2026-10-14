---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: privileged and hostPID are node access wearing a pod's clothes
aha: They read like performance or debugging options in a manifest. Each is really a request to stop being a container, and granting one is granting the node.
revealLabel: why it slips through
diagram:
  caption: reject the switches at the door
  shapes:
    - {type: box, x: 55, y: 66, w: 250, h: 58, label: 'privileged + hostPID', sub: 'before - node is open', tone: danger, delay: 0}
    - {type: wire, from: {x: 180, y: 124}, to: {x: 180, y: 169}, delay: 0.4}
    - {type: box, x: 55, y: 169, w: 250, h: 58, label: 'admission forbids both', sub: 'after - node is sealed', tone: accent, delay: 0.7}
    - {type: seal, cx: 470, cy: 148, r: 30, delay: 1.0}
    - {type: label, x: 470, y: 212, text: 'the wall stays up', tone: accent, delay: 1.2}
---
The `workstation` pod was allowed `privileged` and `hostPID`. Enforce
`PodSecurity` at `baseline` or `restricted` on namespaces that run general
workloads - both levels forbid these settings, so the pod is never admitted with
them. Reserve them, if ever, for a tiny set of audited system components.

Two things worth carrying away. Container isolation is a property of the pod spec,
not a guarantee - `privileged`, `hostPID`, `hostNetwork`, and a `hostPath` mount
each dissolve part of it, so the security review that matters happens at
admission. And when you assess a cluster, the dangerous pods announce themselves
in their own `securityContext`; the escape is written in the spec, waiting to be
read.
