---
kind: concept
ey: CONCEPT 3
eyt: Signal, not noise
tag: technique
title: A port filter turns every connection into just the ones that matter
aha: A cluster dials out constantly - DNS, the API server, the kubelet. A good policy does not drown the analyst in that; it pins the alert to the destination that means "beacon".
diagram:
  caption: match the C2 port, ignore the ambient traffic
  shapes:
    - {type: box, x: 45, y: 100, w: 175, h: 50, label: 'connect :53 / :443', sub: 'ambient - ignored', delay: 0}
    - {type: box, x: 45, y: 165, w: 175, h: 50, label: 'connect :4444', sub: 'C2 - matched', tone: danger, delay: 0.4}
    - {type: wire, from: {x: 220, y: 190}, to: {x: 340, y: 165}, tone: danger, delay: 0.7}
    - {type: box, x: 340, y: 138, w: 180, h: 54, label: 'alert + flag', sub: 'posted', tone: accent, delay: 0.95}
---
Left wide open, a `tcp_connect` hook is noisy - a running cluster opens outbound
connections constantly for DNS, the API server, and the kubelet. Posting all of
them buries the one you care about.

So the policy narrows with a `DPort` selector: it matches only connections whose
destination port is the C2 port, `4444`, and ignores the rest. That is why, when
you open the `Defender` pane, you will not see the cluster's ambient traffic -
only a beacon to `:4444` posts an event. The flag rides that event as its
`message`, so it appears exactly when you make the connection the policy is
looking for, and not before.
