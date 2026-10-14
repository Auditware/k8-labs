---
kind: concept
ey: RECAP
eyt: And the fix
chip:
  label: TAKEAWAY
  tone: accent
title: Read patch-workload as code execution as that workload
aha: It slips through review as operational access - who can roll a deploy. But a workload spec is a program description, and whoever can edit it decides what program runs under that app's identity.
revealLabel: why it slips through
diagram:
  caption: scope patch, and inspect what pods run
  shapes:
    - {type: box, x: 55, y: 66, w: 250, h: 58, label: 'patch any workload', sub: 'before - inject anywhere', tone: danger, delay: 0}
    - {type: wire, from: {x: 180, y: 124}, to: {x: 180, y: 169}, delay: 0.4}
    - {type: box, x: 55, y: 169, w: 250, h: 58, label: 'scoped + admission', sub: 'after - specs are checked', tone: accent, delay: 0.7}
    - {type: seal, cx: 470, cy: 148, r: 30, delay: 1.0}
    - {type: label, x: 470, y: 212, text: 'no uninvited containers', tone: accent, delay: 1.2}
---
The foothold could `patch` a Deployment whose pods held production access. Grant
workload-edit rights narrowly - to the specific workloads a role owns - and add
admission control that inspects pod specs, so an injected `container` with a
foreign image or command is rejected before it ever rolls.

Two things worth carrying away. Any verb that edits a workload - `patch`,
`update` on deployments, daemonsets, statefulsets - is code execution under that
workload's identity, so weigh it that way. And injection hides in plain sight: a
rogue `sidecar` looks like just another container, which is exactly why watching
what actually runs, not just who can deploy, is the control that catches it.
