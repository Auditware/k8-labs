---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You want your container in the payments pod. Can you just add it to the running `Pod`?
opts:
  - t: Yes - edit the pod and add a container
  - t: No - a running pod's containers are fixed; patch the `Deployment` instead
    correct: true
  - t: Only if the pod is restarted first by its owner
  - t: No, containers can never be added to a workload
explain: It feels like you should edit the pod directly, but a running pod's container list is immutable - Kubernetes will not let you add one. The workload that owns it, the `Deployment`, is fully editable, and rolling it recreates the pods with your container baked in. You do not change the pod; you change the thing that makes pods.
---
