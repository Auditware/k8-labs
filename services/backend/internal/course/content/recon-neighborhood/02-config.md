---
kind: concept
ey: CONCEPT 2
eyt: Reading the map
tag: technique
title: The app's own config names its neighbours
aha: Applications carry a map of the services they talk to. Land in the app, and you inherit its map.
---
The workload you landed in was configured to talk to other services, and that
configuration is mounted right beside you. Read it: `$ cat /etc/app/config.yaml`.

Among the `upstreams` is an internal metrics endpoint, named by its cluster
address: `metrics.observability.svc.cluster.local`. You could not `list` that
`Service` over the API - but its own config just handed you the name for free.
That is the thread to pull.
