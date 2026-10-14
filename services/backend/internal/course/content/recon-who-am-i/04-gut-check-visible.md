---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: Back in First Contact you read the pod's environment, mounts, and token files. Could that local inventory have revealed this billing permission?
opts:
  - t: No - a grant lives in the API server's `RBAC`, not anywhere on the pod's disk
    correct: true
  - t: Yes - the permission is written into the mounted `token`
  - t: Yes - it would appear as an environment variable
  - t: Only if the pod mounted the RoleBinding as a file
explain: The `token` proves who you are, but it carries none of what you are allowed to do - that lives entirely in the API server. Reading the pod tells you your identity; only asking the authorizer (a `SelfSubjectRulesReview`) tells you your power. The two recon moves are complementary, not redundant.
---
