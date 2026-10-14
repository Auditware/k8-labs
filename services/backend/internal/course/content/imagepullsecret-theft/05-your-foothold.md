---
kind: concept
ey: THE LAB
eyt: Your starting position
tag: go and look
title: Here is what you have been given
aha: No escalation, no pivot. List what you can read, notice the pull secret among it, and decode your way to the password.
revealLabel: how to start
diagram:
  caption: list secrets, decode the pull credential
  shapes:
    - {type: box, x: 40, y: 116, w: 155, h: 66, label: 'foothold', sub: 'can read Secrets', delay: 0}
    - {type: wire, from: {x: 195, y: 149}, to: {x: 300, y: 149}, delay: 0.4}
    - {type: box, x: 300, y: 116, w: 160, h: 66, label: 'regcred', sub: 'decode x2', tone: accent, delay: 0.7}
    - {type: wire, from: {x: 460, y: 149}, to: {x: 510, y: 149}, tone: accent, delay: 0.95}
    - {type: box, x: 510, y: 116, w: 100, h: 66, label: 'password', sub: 'the flag', tone: accent, delay: 1.15}
---
Your session drops you into a pod called `foothold` in the `lab` namespace. Its
`station` service account can read Secrets there, and `kubectl` uses that token
automatically.

List what is readable - `$ kubectl get secrets` - and find the one of type
`kubernetes.io/dockerconfigjson`, named `regcred`. Pull its data:
`$ kubectl get secret regcred -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d`
gives you the JSON. Read the `auth` field out of it, `base64 -d` that too, and you
have `deploy-bot:` followed by the password. The password is the flag - the
registry account you just walked off with.
