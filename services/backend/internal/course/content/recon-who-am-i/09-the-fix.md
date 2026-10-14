---
kind: concept
ey: DEFEND
eyt: The fix
tag: blue team
title: Broad RBAC answers honestly to anyone who asks
aha: The same endpoint that helps a defender audit access hands an attacker a to-do list. The only real defence is having less to confess.
---
A `SelfSubjectRulesReview` is not a vulnerability - it is a feature, and
defenders should use it too. The problem was the standing grant it revealed: a
`Role` in `billing` bound to a `dev` service account that had no business there.

Scope permissions to the namespace and identity that actually need them, and
review `RoleBinding` objects for subjects from other namespaces. An honest
authorizer is only as safe as the grants it has to admit to.
