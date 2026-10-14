---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: Why is `create` on `mutatingwebhookconfigurations` close to cluster-admin, even with no other grant?
opts:
  - t: It is not - you still need read access to do anything
  - t: A mutating webhook can rewrite every admitted object, so it can inject into any pod cluster-wide
    correct: true
  - t: It only lets you read webhook configs, not create real ones
  - t: It just restarts the API server
explain: It looks like a narrow, niche permission. But a mutating webhook sits in the path of every create, and it can rewrite what it sees - so it can add a container (or change a securityContext, or a service account) on pods across the whole cluster. Controlling admission is controlling everything that passes through it, which is why this grant belongs in the same tier as cluster-admin.
---
