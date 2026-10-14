---
kind: concept
ey: CONCEPT 3
eyt: Placing yourself
tag: technique
title: resolv.conf tells you where you are standing
aha: The DNS config is a map pin. Its search domains spell out your namespace and the cluster's domain, for free.
---
`$ cat /etc/resolv.conf`. Kubernetes writes it for every pod, and its `search`
line is a giveaway: it lists `<your-namespace>.svc.cluster.local`,
`svc.cluster.local`, and `cluster.local`. That first entry names the namespace
you are in, and confirms the cluster's DNS domain.

Between the `namespace` file and `resolv.conf`, you now know who you are and
where you stand - still without making a single request off the pod.
