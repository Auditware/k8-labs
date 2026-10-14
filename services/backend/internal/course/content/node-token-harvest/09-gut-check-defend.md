---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: The `metrics-agent` genuinely needs its access. What best limits this attack?
opts:
  - t: Rotate the flag Secret more often
  - t: Keep pods off the node's filesystem at admission, and stop mounting unused tokens
    correct: true
  - t: Give the `station` account read access so it stops harvesting
  - t: Run `metrics-agent` with more replicas
explain: The agent needs its access, and rotating the secret or scaling it changes nothing. The harvest was possible because a pod could read the node's disk and because a powerful token was sitting there mounted. Block node access at admission (no `hostPath`, no privileged pods), and stop mounting the `token` on pods that do not call the API, so fewer strong tokens are lying on the node to steal.
---
