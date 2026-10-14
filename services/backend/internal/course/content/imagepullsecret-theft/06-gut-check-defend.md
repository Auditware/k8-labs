---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: The workload genuinely needs its image pulled from the private registry. What best cuts this theft?
opts:
  - t: Delete the pull secret - the pod can manage without it
  - t: Do not grant the workload's account read on Secrets, and scope the pull credential to pull-only
    correct: true
  - t: Base64-encode the password a third time
  - t: Move the Secret to another namespace
explain: The pod does not need to *read* the pull secret - the kubelet uses it to pull, referenced by name, without the container ever seeing it. So the fix is to not hand the workload's own account `get secrets` at all (least privilege on the identity), and to make the credential itself least-privilege - a pull-only, per-repository registry token, so even a stolen one cannot push a poisoned image. Deleting it breaks pulls; more base64 protects nothing; moving namespaces just moves the target.
---
