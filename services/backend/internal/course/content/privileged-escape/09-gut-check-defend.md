---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: Which control stops this breakout?
opts:
  - t: Remove the pod's API permissions
  - t: Block `privileged` and `hostPID` at admission
    correct: true
  - t: Run the container as a non-root user
  - t: Put the pod in its own namespace
explain: API permissions never mattered - the read was on the node's filesystem. Running as non-root helps a little but a `privileged` pod can regain what it needs, and a namespace change does nothing. The real fix is admission - PodSecurity's baseline and restricted levels forbid both `privileged` and `hostPID`, so the pod is rejected before it can ever be created with those switches.
---
