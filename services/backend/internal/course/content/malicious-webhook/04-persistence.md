---
kind: concept
ey: CONCEPT 3
eyt: Why it is persistence
tag: the machinery
title: The webhook keeps working after you are locked out
aha: Kick the attacker out, rotate their token, delete their pod - and every new deployment still comes up backdoored, because the trap is in the gate, not in them.
diagram:
  caption: the config outlives the attacker
  shapes:
    - {type: box, x: 55, y: 116, w: 165, h: 66, label: 'attacker gone', sub: 'token revoked', delay: 0}
    - {type: wire, from: {x: 220, y: 149}, to: {x: 330, y: 149}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 330, y: 116, w: 225, h: 66, label: 'webhook still fires', sub: 'every new pod', tone: danger, delay: 0.7}
---
Most footholds are fragile - revoke the identity and the access is gone. A
webhook is not like that. Once the `MutatingWebhookConfiguration` exists, it is
part of the cluster's admission pipeline, independent of whoever created it. The
attacker's shell can be closed and their account disabled, and every pod created
afterwards is still rewritten.

That is the definition of `persistence`: durable access that survives losing your
initial foothold. And because the injected container is just one more sidecar
among many legitimate ones, it hides in a crowd - a mesh proxy, a log shipper, and
your backdoor all look alike in a pod spec.
