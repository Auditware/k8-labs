---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: Which control actually stops this?
opts:
  - t: Stricter RBAC on the `API server`
  - t: Keep pods off the node's filesystem at admission, so none can write the kubelet's directory
    correct: true
  - t: A `PodSecurity` policy on the static pod
  - t: Rotating the flag file
explain: RBAC and `PodSecurity` guard the `API server`, and a `static pod` never goes there - so tightening them does nothing, and you cannot put a policy on a pod the policy engine never sees. The only lever is upstream - stop any pod from reaching the node's filesystem in the first place (no `hostPath`, enforced at admission), so nothing can write the kubelet's directory to begin with.
---
