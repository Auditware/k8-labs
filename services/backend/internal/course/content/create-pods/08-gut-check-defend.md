---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: The `builder` account genuinely needs to read production secrets for its job. What best stops this attack?
opts:
  - t: Delete `builder` - the CI job can manage without it
  - t: Scope the foothold's `create pods` grant, or add admission that restricts serviceAccountName
    correct: true
  - t: Give the foothold read access so it stops creating pods
  - t: Put `builder` and the foothold in the same namespace
explain: The `builder` account needs its access, so removing it breaks the job, and granting the foothold more access only makes things worse. The escalation came from `create pods` letting a weak account launch a pod as `builder`. Remove or tightly scope that grant, or enforce an admission policy (which accounts a creator may assign) - that closes the path while `builder` keeps working.
---
