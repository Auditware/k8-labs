---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: The `API server` refuses to let your account create a `privileged` pod. Can you still get one running on the node?
opts:
  - t: No - if the API server refuses, the pod cannot exist
  - t: Yes - write it as a `static pod` in the kubelet's directory, which skips that check
    correct: true
  - t: Only if an admin approves it first
  - t: Only by editing the API server's config
explain: The refusal feels final, but it only covers the front door. The `API server` gates pods created through it; a `static pod` is run by the kubelet from disk, so its `admission` check never happens. If you can write the kubelet's directory, "the API server said no" simply does not apply to you.
---
