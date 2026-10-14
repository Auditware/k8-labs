---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: The payments app genuinely needs its production access. What best cuts this attack?
opts:
  - t: Remove payments' secret access - the app will just break
  - t: Scope who can `patch` the payments `Deployment`, and alert on unexpected containers
    correct: true
  - t: Give the foothold read access so it stops patching
  - t: Rotate the payments token nightly
explain: Payments needs its access, and granting the foothold more only widens the blast radius. The attack came from a weak account being able to `patch` the workload and inject a container. Restrict that patch grant, and watch workload specs for containers whose image or command do not belong - admission policy can reject them. Payments keeps working; the injection path closes.
---
