---
kind: concept
ey: CONCEPT 1
eyt: Who the pod is
tag: technique
title: Your identity is a file, not a login
aha: You never typed a password. The pod authenticates with a file on disk - which means anyone standing in the pod is already that identity.
---
A pod does not log in. Kubernetes mounts its identity as files under
`/var/run/secrets/kubernetes.io/serviceaccount/` - a `token`, the cluster
`ca.crt`, and a `namespace` file naming where you are.

Read the `namespace` file first with `$ cat /var/run/secrets/kubernetes.io/serviceaccount/namespace`:
it tells you which namespace you are standing in without a single API call. The
`token` is the pod's `ServiceAccount`
credential - you will not spend it in this lab, but knowing it is there, on
disk, is the point. Identity in Kubernetes is a file you can read, not a session
you have to crack.
