---
kind: concept
ey: CONCEPT 4
eyt: Borrowing a group
tag: the machinery
title: You can borrow a group, not just a user
aha: A user is one identity to guess at. A group is a role thousands of accounts already share - and the cluster ships with some very powerful ones.
diagram:
  caption: groups add reach the user alone never had
  shapes:
    - {type: box, x: 60, y: 112, w: 175, h: 66, label: 'kubectl --as=x', sub: '--as-group=g', delay: 0}
    - {type: wire, from: {x: 235, y: 145}, to: {x: 355, y: 145}, tone: danger, delay: 0.4}
    - {type: box, x: 355, y: 112, w: 220, h: 66, label: 'evaluated as', sub: 'user x, in group g', tone: danger, delay: 0.7}
    - {type: label, x: 317, y: 205, text: 'the group carries the privilege', delay: 1.0}
---
Impersonation is not limited to users. If the `impersonate` verb was granted on
`groups`, you can add `$ kubectl --as-group` to name one or more groups the
impersonated identity should be treated as belonging to.

This matters because authorization sums over *all* of an identity's groups. A
plain user with a powerful group attached is authorised as that group. So the
interesting question stops being "which user can I become" and becomes "which
group can I claim".
