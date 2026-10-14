---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: The privileged workload is already running. You are just adding a quiet passenger that reads what the driver can read.
revealLabel: how to start
diagram:
  caption: inject into payments, read the sidecar's logs
  shapes:
    - {type: box, x: 45, y: 116, w: 150, h: 66, label: 'foothold', sub: 'patch only', delay: 0}
    - {type: wire, from: {x: 195, y: 149}, to: {x: 295, y: 149}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 295, y: 116, w: 170, h: 66, label: 'payments pod', sub: '+ your sidecar', tone: danger, delay: 0.7}
    - {type: wire, from: {x: 465, y: 149}, to: {x: 535, y: 110}, tone: accent, delay: 0.95}
    - {type: box, x: 510, y: 60, w: 110, h: 50, label: 'flag', sub: 'production', tone: accent, delay: 1.15}
---
Your session drops you into a pod called `foothold` in the `dev` namespace. Its
account can `patch` Deployments and read pods and logs in `ops` - confirm with
`$ kubectl auth can-i --list -n ops`. **The namespace is the whole trick here:**
your grant is scoped to `ops`, so a bare `--list` (which checks only your own
`dev` namespace) or an `-A` / all-namespaces check both report nothing - that is
not a broken lab, it means you have to name the namespace.

In `ops` there is a `Deployment` named `payments` whose pods can read the `flag`
Secret in `production`. Find it (`$ kubectl get deployments -n ops`), then patch
it to add a `container` - image `k8labs-foothold:1.31`, `imagePullPolicy` set to
`Never` - running a command that reads the `flag` and prints it. When the
Deployment rolls, your `sidecar` comes up inside the payments pod, holding its
`token`. Read that container's `logs` and decode the value. You never held a read
grant - your passenger did.
