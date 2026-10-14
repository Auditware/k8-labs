---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You can `create pods` in a namespace. What stops you setting `serviceAccountName` to a powerful account you do not own?
opts:
  - t: A separate "use this ServiceAccount" permission is required
  - t: Nothing, in plain RBAC - the field is yours to fill in
    correct: true
  - t: The pod is rejected unless you are that account
  - t: Only cluster-admins may set serviceAccountName
explain: It feels like there should be a gate - surely you cannot just wear any badge. In plain RBAC there is not - `create pods` includes setting every field of the pod, `serviceAccountName` among them. Some clusters add an admission policy to restrict it, but nothing in RBAC does. The create grant already carried the escalation.
---
