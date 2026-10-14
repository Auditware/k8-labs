---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: The `ci-runner` pod legitimately needs to read production secrets. Which change best cuts the attack without breaking ci?
opts:
  - t: Remove ci's secret access - it will just break the CI job
  - t: Remove the foothold's `pods/exec` grant on `ops`
    correct: true
  - t: Move ci-runner to a different namespace
  - t: Rotate the flag Secret on a schedule
explain: ci needs its access, so removing it breaks the job. Moving the pod or rotating the secret changes nothing about who can exec in. The attack existed because a weak account could `exec` into ci's pod and borrow its identity - take that exec grant away and the powerful pod is no longer reachable, while ci keeps doing its job.
---
