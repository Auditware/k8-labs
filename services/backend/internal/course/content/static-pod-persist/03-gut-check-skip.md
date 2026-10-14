---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: A `static pod` is run by the kubelet straight from disk. Which of the cluster's checks does it skip?
opts:
  - t: None - it is still validated, just more slowly
  - t: Both RBAC and `admission`, because the `API server` never sees the create
    correct: true
  - t: Only RBAC, but admission still applies
  - t: Only logging, not any security checks
explain: It is natural to assume every pod is checked somewhere. But RBAC and `admission` live in the `API server`, and a static pod is started by the kubelet before the API server is ever involved - the mirror pod appears only after it is already running. So neither gate applies. Whatever the manifest asks for, it gets.
---
