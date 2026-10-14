---
kind: concept
ey: CONCEPT 1
eyt: How access is granted
tag: foundation
title: Permissions come from a binding, not from a role
aha: A role is a list of powers sitting on a shelf. It does nothing until a binding takes it down and hands it to someone.
diagram:
  caption: role lists powers; binding gives them to a subject
  shapes:
    - {type: box, x: 45, y: 116, w: 150, h: 66, label: 'ClusterRole', sub: 'a list of powers', delay: 0}
    - {type: wire, from: {x: 195, y: 149}, to: {x: 285, y: 149}, delay: 0.4}
    - {type: box, x: 285, y: 116, w: 145, h: 66, label: 'RoleBinding', sub: 'gives it to X', tone: accent, delay: 0.7}
    - {type: wire, from: {x: 430, y: 149}, to: {x: 510, y: 149}, tone: accent, delay: 0.9}
    - {type: box, x: 510, y: 116, w: 100, h: 66, label: 'account X', delay: 1.1}
---
In Kubernetes, a `Role` or a `ClusterRole` is only a list of allowed actions. It
grants nothing by itself. What actually gives an account power is a `RoleBinding`
- an object that says "this subject gets the rules in that role, here".

So the power to write bindings is the power to decide who holds which role. If you
can create a `RoleBinding`, you are not editing a config file off to the side -
you are operating the mechanism that assigns permission itself.
