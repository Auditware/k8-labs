---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: A reviewer says "the pod's service account has no permissions, so it is safe." Are they right?
opts:
  - t: Yes - no RBAC grants means no access
  - t: No - the `hostPath` mount reads the node directly, with no RBAC involved
    correct: true
  - t: Yes, as long as the token is not mounted
  - t: Only safe if the pod is in its own namespace
explain: The reviewer is checking the wrong door. RBAC and the service account govern API access, and here there is none. But the `hostPath` of `/` gives filesystem access to the `node`, which RBAC never sees. A pod with zero API permissions and a host-root mount can still read the whole machine. Locking the token does nothing.
---
