---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: Your pod mounts a `hostPath` of the node's `/` at `/host`. What can you read?
opts:
  - t: Only the files your own container image shipped
  - t: Only the mounted directory, but not its contents
  - t: Whatever the node can read - other pods' files, node credentials, the flag
    correct: true
  - t: Nothing, unless the pod is privileged
explain: The instinct is that a mount is a narrow, contained thing. But a `hostPath` of `/` maps the node's whole root into your pod, so under `/host` you have the node's filesystem - every neighbouring pod's data and the node's own secrets. You do not even need to be privileged; the mount alone did it.
---
