---
kind: concept
ey: CONCEPT 4
eyt: Binding
chip:
  label: KEY
  tone: accent
title: A RoleBinding grants a Role inside exactly one namespace
aha: The binding is the act of hiring. The job description on the wall did nothing until someone was appointed to it - and appointed in one specific office.
diagram:
  caption: the binding lives in a namespace, so the grant does too
  shapes:
    - {type: box, x: 45, y: 118, w: 150, h: 62, label: 'ServiceAccount', sub: 'analyst', delay: 0}
    - {type: wire, from: {x: 195, y: 149}, to: {x: 255, y: 149}, tone: accent, delay: 0.3}
    - {type: box, x: 255, y: 112, w: 150, h: 74, label: 'RoleBinding', sub: 'in namespace dev', tone: accent, delay: 0.5}
    - {type: wire, from: {x: 405, y: 149}, to: {x: 465, y: 149}, tone: accent, delay: 0.8}
    - {type: box, x: 465, y: 118, w: 135, h: 62, label: 'Role', sub: 'read secrets', delay: 1.0}
    - {type: label, x: 330, y: 214, text: 'grant stops at the namespace edge', tone: accent, delay: 1.2}
---
A `RoleBinding` connects a `subject` - a user, a group, or a `ServiceAccount` - to
a `Role`. Crucially, a RoleBinding is **itself an object inside a namespace**, and
it only grants permission within that namespace.

Bind the analyst account to a secret-reading Role with a RoleBinding in `dev`, and
it can read secrets in dev. In any other namespace it is refused. The fence
holds, because the grant was written inside the fence.
