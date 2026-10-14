---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: Your pod is `privileged` and has `hostPID`. How do you read a root-owned file on the node?
opts:
  - t: You cannot without installing extra tools first
  - t: Read it under `/proc/1/root` - the node init's root is the node's filesystem
    correct: true
  - t: Ask the API server for the node's files
  - t: Only by rebooting the node
explain: The move is not a special exploit tool - it is a path. `hostPID` makes the node init `PID 1`, and `privileged` lets you follow its root, so `/proc/1/root` is the node's filesystem and an ordinary read returns the file. The API server has nothing to do with node files, and no reboot or extra binary is needed - the two settings already opened the door.
---
