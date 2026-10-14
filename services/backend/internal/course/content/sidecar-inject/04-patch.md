---
kind: concept
ey: CONCEPT 3
eyt: The patch verb
chip:
  label: KEY IDEA
  tone: danger
title: patch on a Deployment is code execution inside it
aha: The verb sounds like a maintenance task. It is really the right to decide what programs run under that app's name.
diagram:
  caption: one patch rolls in your container
  shapes:
    - {type: box, x: 55, y: 116, w: 180, h: 66, label: 'patch Deployment', sub: 'add a container', tone: danger, delay: 0}
    - {type: wire, from: {x: 235, y: 149}, to: {x: 350, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 350, y: 116, w: 210, h: 66, label: 'new pods roll', sub: 'your code inside', tone: danger, delay: 0.7}
---
The RBAC verb `patch deployments` lets you modify a Deployment's spec - including
its `Pod` template. Add a `container` to that template and, when the Deployment
rolls, every new pod runs it. You have not asked anyone's permission for what that
container does; you have simply described it, and the cluster runs it as the
workload.

So `patch deployments` is not a config-tweak permission. It is the ability to run
arbitrary code inside a workload, holding that workload's identity. Whoever can
patch a Deployment can make its pods do anything the app's account is allowed to
do.

Note the difference from creating your own pod: there you *choose* the
`serviceAccountName` and can name any account in the namespace. Here you do not
get to choose - you inherit whatever identity the target Deployment already runs
as. That is why this attack only works when the workload you can patch, `payments`,
is itself wired to read the flag; you cannot point it at a stronger account.
