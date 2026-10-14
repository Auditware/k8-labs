---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You get a shell in a pod running as a `ServiceAccount`. What have you gained?
opts:
  - t: Nothing yet - you still need credentials to talk to the cluster
  - t: Whatever that account is allowed to do, immediately
    correct: true
  - t: Full access to the namespace the pod is in
  - t: Read access to the pod's own definition only
explain: The `token` is already mounted in the filesystem, so there is no credential left to steal - you are the account the moment you have the shell. But it grants exactly what that account was permitted, no more. Which raises the real question, and the subject of the next act - who decides what an account is permitted?
---
