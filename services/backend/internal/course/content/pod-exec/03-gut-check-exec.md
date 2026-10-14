---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You `exec` into a pod that runs as a powerful `ServiceAccount`. Whose permissions do your commands now have?
opts:
  - t: Your own - you are still you, just in a different pod
  - t: The pod's - its mounted `token` is right there for you to use
    correct: true
  - t: None, until you supply a password
  - t: Only read access to that pod
explain: The instinct is that you carry your own identity in with you. But `exec` gives you the pod's shell and filesystem, and the pod's `token` is a file inside it. Nothing stops you reading that file and making API calls as the pod. You did not authenticate as yourself in there - you picked up whoever lives there.
---
