---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You never got past the API's `403`. Which layer actually gave you the flag?
opts:
  - t: The network - `DNS` resolution and an open pod-to-pod route, neither gated by `RBAC`
    correct: true
  - t: The API server, once you found the right endpoint
  - t: The `token`, which secretly carried network access
  - t: The `ClusterIP` range, which is public
explain: You never defeated `RBAC` - you went around it. The flag came off the network plane - the app config named a service, `DNS` resolved it, and an unrestricted pod route carried your request. Treating a `list` denial as isolation is the mistake this lab exists to break.
---
