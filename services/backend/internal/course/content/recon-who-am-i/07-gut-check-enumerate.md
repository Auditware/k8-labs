---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: Nobody handed you the name `billing`. How did you come to point a rules review at exactly that namespace?
opts:
  - t: You enumerated every namespace over the API, then probed each one
    correct: true
  - t: It was printed in the pod's environment
  - t: The `token` file listed the namespaces you can reach
  - t: You guessed a common name and got lucky
explain: This is the whole discipline of recon - you did not need to be told where to look, because the API server will list its namespaces and then answer, one by one, what you can do in each. Enumeration turns "I have a `token`" into "I have a map".
---
