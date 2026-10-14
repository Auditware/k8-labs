---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: Your own `ServiceAccount` can read nothing, but you can read the node's disk. What is your real reach?
opts:
  - t: Nothing - your account has no permissions
  - t: As much as the most powerful pod scheduled on this node
    correct: true
  - t: Only the pods in your own namespace
  - t: Read-only access to the node, but no API access
explain: Your account's permissions are a distraction. Node access lets you read every pod's `token` off the disk, and a stolen token is a full credential. So your reach is the reach of the strongest identity on the node, not your own - and node access is very much API access, once you pick up someone else's key.
---
