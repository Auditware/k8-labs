---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: The kubelet's directory is a trust boundary, so guard the node
aha: It reads like an internal implementation detail - a folder the kubelet happens to watch. It is really an ungated path to running anything, and it is only as safe as the node's filesystem.
revealLabel: why it matters
diagram:
  caption: seal the node, watch for uninvited pods
  shapes:
    - {type: box, x: 55, y: 66, w: 250, h: 58, label: 'node writable', sub: 'before - side door open', tone: danger, delay: 0}
    - {type: wire, from: {x: 180, y: 124}, to: {x: 180, y: 169}, delay: 0.4}
    - {type: box, x: 55, y: 169, w: 250, h: 58, label: 'node sealed', sub: 'after - only the front door', tone: accent, delay: 0.7}
    - {type: seal, cx: 470, cy: 148, r: 30, delay: 1.0}
    - {type: label, x: 470, y: 212, text: 'every pod is checked again', tone: accent, delay: 1.2}
---
This attack needed one thing - write access to the node's filesystem, and
specifically the kubelet's static-pod directory. Close it the same way you close
every escape: `PodSecurity` at `admission`, forbidding `hostPath` so no pod can
reach the node's disk. On the node itself, the static-pod directory should be
writable only by root-level components, never by a workload.

Two things worth carrying away. Controls that live in the `API server` - RBAC,
`admission` - protect only what passes through it, and the kubelet's directory is
a way around all of them, so node integrity is its own security problem. And in
detection, treat a mirror pod that no `API server` create produced, or any write
to the kubelet's manifest directory, as a strong signal of exactly this.
