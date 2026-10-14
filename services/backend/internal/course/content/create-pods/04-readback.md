---
kind: concept
ey: CONCEPT 3
eyt: Getting the answer out
tag: the machinery
title: You will not get a shell - so make the pod tell you
aha: You are not climbing into the pod. You are sending it in with instructions and reading the note it leaves behind.
diagram:
  caption: the pod reads the secret; you read its logs
  shapes:
    - {type: box, x: 50, y: 116, w: 175, h: 66, label: 'your pod', sub: 'runs as builder', tone: danger, delay: 0}
    - {type: wire, from: {x: 225, y: 149}, to: {x: 320, y: 149}, tone: accent, delay: 0.4}
    - {type: box, x: 320, y: 116, w: 130, h: 66, label: 'secret', sub: 'printed', tone: accent, delay: 0.7}
    - {type: wire, from: {x: 450, y: 149}, to: {x: 525, y: 110}, delay: 0.9}
    - {type: box, x: 500, y: 60, w: 115, h: 50, label: 'kubectl logs', delay: 1.1}
---
Creating a pod is not the same as execing into one - here you hold `create`, not
exec, so you never get an interactive shell inside it. That is fine. Give the pod
a command that does the work for you: read the secret and print it to standard
output.

Whatever a pod writes to stdout becomes its log, and `$ kubectl logs` retrieves
it (you were granted `pods/log` for exactly this). So the plan is: launch a pod
as the powerful account, have it read the secret and echo it, then read the
pod's `logs`.
