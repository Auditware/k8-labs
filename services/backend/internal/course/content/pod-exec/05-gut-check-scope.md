---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You hold `pods/exec` in the `ops` namespace. Your own account can read no secrets. How much can you reach?
opts:
  - t: Nothing - your account has no read grants
  - t: Only pods that run as your own `ServiceAccount`
  - t: Whatever the most privileged pod in `ops` can reach
    correct: true
  - t: Every namespace in the cluster
explain: Your own grants are a red herring. `exec` lets you inherit any identity running in `ops`, so your reach is the reach of the strongest pod there - not your own. It is not the whole cluster either - you can only become identities whose pods you can exec into. Find the strongest one.
---
