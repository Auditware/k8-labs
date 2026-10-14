---
kind: concept
ey: DEFEND
eyt: The fix
tag: blue team
title: A list denial is a lock on the door, not the walls
aha: Hiding a service from the API does nothing if any pod can still route to it. Isolation is a network property, not an RBAC one.
---
Withholding `list` on `services` only hides them from the API - it never stops a
pod from resolving and reaching them. Real isolation is a `NetworkPolicy`:
default-deny ingress, then allow only the workloads that genuinely need each
service.

And the metrics endpoint should not have trusted the network in the first place.
Unauthenticated internal services assume the pod network is a safe zone; a single
compromised workload is all it takes to prove it never was.
