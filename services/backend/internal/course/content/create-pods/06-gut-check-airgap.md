---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: Your first pod, built from `nginx`, is stuck in `ImagePullBackOff`. What actually went wrong?
opts:
  - t: Your `create pods` grant was revoked
  - t: The cluster is airgapped and cannot pull `nginx` - use a local image
    correct: true
  - t: The `builder` account lacks permission to run nginx
  - t: The pod needs more memory
explain: It is easy to read a stuck pod as a permissions problem, but the create succeeded - the pod exists, it just cannot start. `ImagePullBackOff` is the cluster failing to fetch the image, because it pulls nothing. Point the pod at `k8labs-foothold:1.31` with `imagePullPolicy` set to `Never` and it boots from the local copy.
---
