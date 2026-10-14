---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You captured a credential from the pod's environment. How much network traffic did the capture require?
opts:
  - t: None - reading `env` is a local operation, invisible on the wire
    correct: true
  - t: One request to the API server
  - t: A connection to the database the password unlocks
  - t: A DNS lookup for the `Secret`
explain: This is what makes environment-variable leaks so quiet. A `secretKeyRef` value is read from local memory; no packet leaves the pod, so no network monitor, NetworkPolicy, or audit log ever sees the capture. The only defence is not putting the secret in `env` to begin with.
---
