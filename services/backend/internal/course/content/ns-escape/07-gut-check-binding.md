---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You bind a `ClusterRole` that allows reading secrets. Which choice keeps the grant inside one namespace?
opts:
  - t: Neither - a `ClusterRole` always grants cluster-wide
  - t: A `ClusterRoleBinding`, because the role is cluster-scoped
  - t: A `RoleBinding` in that namespace
    correct: true
  - t: Either one, as long as the role lists only one namespace
explain: The role is just the list of permissions and carries no scope of its own - a `ClusterRole` bound by a `RoleBinding` grants only inside that binding's namespace, which is a completely normal and safe pattern. The reach comes from the binding. And a `Role` cannot name namespaces at all, so the last option describes something that does not exist.
---
