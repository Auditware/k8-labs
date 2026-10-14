---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: The API returned `403` when you tried to `list` services. So how can you still find and reach one?
opts:
  - t: DNS and the pod network are separate planes that never consult `RBAC`
    correct: true
  - t: The `403` only blocks writes, not reads
  - t: Listing works if you retry with the `token` in a cookie
  - t: You cannot - the service is genuinely unreachable
explain: Authorization gates the API server. Name resolution and pod-to-pod networking are different systems entirely, and by default neither asks the authorizer anything. A `list` denial hides the service from one interface while leaving it wide open on another.
---
