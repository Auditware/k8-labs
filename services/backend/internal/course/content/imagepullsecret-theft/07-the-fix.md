---
kind: concept
ey: THE FIX
eyt: How to close it
tag: defence
title: Least privilege on the identity, and on the credential
aha: Two least-privilege moves close it - nobody who does not need to read Secrets should, and the pull credential should be too weak to do anything but pull.
diagram:
  caption: both sides small
  shapes:
    - {type: box, x: 45, y: 116, w: 215, h: 66, label: 'no get secrets', sub: 'for workload SAs', tone: accent, delay: 0}
    - {type: box, x: 300, y: 116, w: 245, h: 66, label: 'pull-only registry token', sub: 'per-repo, cannot push', tone: accent, delay: 0.6}
---
The theft needed two things, and each has a fix.

It needed an identity that could read Secrets. A workload almost never needs
`get secrets` on its namespace - the kubelet mounts what a pod needs, and uses the
pull secret by reference without the container reading it. So do not grant it:
keep Secret read off workload service accounts, and audit the wildcard roles that
hand it out by accident.

And it needed the credential to be worth stealing. Issue the registry credential
as a pull-only, per-repository token, not a full account - most registries
support this. Then even a harvested credential cannot push a poisoned image; the
worst it does is pull what the cluster could already pull. Small identity, small
credential: the blast radius of reading one Secret drops to nearly nothing.
