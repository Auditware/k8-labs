---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: Your injected `sidecar` boots inside the payments pod. Which identity can it use to call the API?
opts:
  - t: Your own foothold identity, since you created it
  - t: The payments pod's `ServiceAccount`, whose `token` is mounted in the pod
    correct: true
  - t: No identity - injected containers get none
  - t: A new anonymous identity
explain: It is tempting to think the container carries your identity in with it. It does not - identity in Kubernetes lives on the pod, not on who patched it. The `sidecar` shares the payments pod, so it reads the payments `token` mounted there and acts as payments. You supplied the code; the pod supplied the credentials.
---
