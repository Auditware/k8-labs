---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You hold `create rolebindings`. You write a binding granting `cluster-admin` to yourself. What happens?
opts:
  - t: You become cluster-admin immediately
  - t: The API server rejects it - you cannot grant what you do not already hold
    correct: true
  - t: It works, but only for reads
  - t: The binding is created but silently ignored
explain: The intuitive answer is that create-rolebindings is game over on its own. It is not. Kubernetes has a guard against exactly this move - to create a binding, you must already hold every permission it grants, or the request is refused. Your naive self-grant is blocked. The lab exists because there is a way around that guard.
---
