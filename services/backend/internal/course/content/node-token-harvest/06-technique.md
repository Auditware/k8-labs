---
kind: concept
ey: CONCEPT 4
eyt: Finding the strongest key
tag: technique
title: Harvest the tokens, then ask each what it can do
aha: You do not have to guess which token is powerful. Every token will tell you its own permissions, if you just ask the cluster while holding it.
diagram:
  caption: test each token against the API
  shapes:
    - {type: box, x: 55, y: 118, w: 190, h: 64, label: 'each token file', sub: 'found on the node', delay: 0}
    - {type: wire, from: {x: 245, y: 150}, to: {x: 355, y: 150}, tone: accent, delay: 0.4}
    - {type: box, x: 355, y: 118, w: 200, h: 64, label: 'auth can-i', sub: 'what can this one do', tone: accent, delay: 0.7}
---
Through your host mount, the token files sit under the node's
`/var/lib/kubelet/pods` tree - `$ find` will list them. Each is a plain file
holding one account's `token`.

To find the one worth using, ask each in turn. `kubectl auth can-i`, run with a
given `$ kubectl --token`, reports that account's permissions. Loop over the
harvested tokens, check which can read secrets, and you have identified your
target without a single guess. The cluster does the sorting for you.
