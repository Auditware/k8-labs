---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: Which control actually prevents this escape?
opts:
  - t: Tighten the pod's RBAC and unmount its token
  - t: Block pods from using `hostPath` mounts at admission
    correct: true
  - t: Encrypt the flag file on the node
  - t: Move the workload to another namespace
explain: RBAC and tokens are irrelevant - the escape never touched the API. Encryption or relocation do not help either, since the node access is total. The only real defence is refusing the dangerous pod shape at admission - PodSecurity's `baseline`/`restricted` levels and policy engines block `hostPath` mounts outright, so the pod can never be created with that door in it.
---
