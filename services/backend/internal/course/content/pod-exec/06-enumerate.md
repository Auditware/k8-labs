---
kind: concept
ey: CONCEPT 4
eyt: Finding the target
tag: technique
title: List the pods, then read what each one is
aha: The cluster will happily tell you which pods run as which accounts. The reconnaissance is not sneaking - it is reading a directory that is open to you.
diagram:
  caption: the pod list names each pod's account
  shapes:
    - {type: box, x: 60, y: 118, w: 195, h: 64, label: 'kubectl get pods', sub: '-n ops -o wide', delay: 0}
    - {type: wire, from: {x: 255, y: 150}, to: {x: 365, y: 150}, delay: 0.4}
    - {type: box, x: 365, y: 118, w: 215, h: 64, label: 'pod -> serviceAccount', sub: 'read the spec', tone: accent, delay: 0.6}
---
Start where every foothold starts: ask what you hold - but ask about the right
namespace. `$ kubectl auth can-i --list -n ops` shows your `pods/exec` grant
there; a bare `$ kubectl auth can-i --list` checks only your own `dev` namespace,
where you can do nothing - permissions are answered per namespace. Then turn to
the namespace you can reach: `$ kubectl get pods -n ops` lists the pods, and
reading a pod's spec shows which `ServiceAccount` it runs as.

You are looking for the pod whose account is worth borrowing - the one wired to
read something you cannot. Once you have its name, you have your way in.
