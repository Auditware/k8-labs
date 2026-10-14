---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: An account can `impersonate` `groups` but has no read access of its own. Why is `system:masters` game over?
opts:
  - t: It grants a Role that can read every namespace
  - t: It is bound to cluster-admin by a default binding the cluster won't let you remove, so the account's own grants are irrelevant
    correct: true
  - t: It only works if the account is also in the production namespace
  - t: It lets you reset the cluster-admin password
explain: The trap is to picture `system:masters` as a very permissive Role you might tighten. It is not something you can narrow - it is the group bound to `cluster-admin` by a protected, auto-reconciled `ClusterRoleBinding`, cluster-wide. The impersonating account never needed a read grant of its own; being able to claim the group is the entire attack.
---
