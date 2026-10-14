---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: A container is `privileged`. How much does that alone separate it from root on the node?
opts:
  - t: Fully - privileged containers are still strongly isolated
  - t: Very little in terms of power, though its own namespaces still hide some things
    correct: true
  - t: Nothing changes; privileged is only about storage
  - t: It only affects networking
explain: It is comforting to think a container stays contained even when privileged. It does not - the flag restores the powers that made it a container. What is left is only the namespace isolation (its own view of processes, network), which hides things but grants no protection once you also share one of those namespaces. Power is already gone; visibility is the last thing standing.
---
