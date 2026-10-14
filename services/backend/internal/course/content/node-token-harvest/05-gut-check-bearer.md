---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You read another pod's `token` file off the node. What must you do to use it as that account?
opts:
  - t: Nothing extra - present the `token` and the API server treats you as that account
    correct: true
  - t: Crack it, since tokens are encrypted per pod
  - t: Run your shell inside the other pod first
  - t: Ask the API server to transfer ownership
explain: The instinct is that a token must be tied to its pod somehow. It is not - it is a bearer credential, so possession is the whole of it. Present it with `--token` and you are the account, no cracking, no ownership transfer, no need to be inside the original pod. That is exactly why a token on disk is as good as the account itself.
---
