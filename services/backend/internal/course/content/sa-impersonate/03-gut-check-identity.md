---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: Your `ServiceAccount` `sa-imper` is granted no read access to anything. How much can you read?
opts:
  - t: Nothing - no read grant means no reads
  - t: Only resources in your own namespace
  - t: Potentially everything, depending on what *other* verbs it holds
    correct: true
  - t: Only the pod's own definition
explain: It is tempting to equate "no read grant" with "cannot read". But authorization is per-request, and a verb that lets you change *whose* request it is can route around your own missing grants entirely. What matters next is not what `sa-imper` may read, but what else it may do.
---
