---
kind: concept
ey: CONCEPT 2
eyt: The create-binding verb
tag: the machinery
title: Creating a binding is choosing who gets a role
aha: Nothing in a binding says the subject has to be someone else. You are allowed to write your own name in the box.
diagram:
  caption: you write the subject line
  shapes:
    - {type: box, x: 55, y: 108, w: 220, h: 90, label: 'create RoleBinding', sub: 'role: X, subject: me', tone: danger, delay: 0}
    - {type: wire, from: {x: 275, y: 153}, to: {x: 385, y: 153}, tone: danger, delay: 0.5}
    - {type: box, x: 385, y: 120, w: 185, h: 66, label: 'you now hold X', tone: danger, delay: 0.8}
---
The RBAC verb `create rolebindings` lets you make new bindings. A binding names
two things: which role, and which subject receives it. Both are fields you fill
in when you create it.

The subject can be any account - including your own. So on paper,
`create rolebindings` looks like the power to grant yourself any role in the
cluster, `cluster-admin` included - write a binding whose role is powerful and
whose subject is you. If that were the whole story, this lab would be one command
long. It is not, because of
what the next screen introduces.
