---
kind: concept
ey: CONCEPT 6
eyt: The real lesson
chip:
  label: KEY
  tone: accent
title: What actually separates two namespaces is the bindings you wrote
aha: The fence in the diagram was never load-bearing. What held things apart was a decision recorded in a binding - and a decision can be typed wrong.
revealLabel: the takeaway
diagram:
  caption: the boundary is a policy, not a wall
  shapes:
    - {type: box, x: 70, y: 105, w: 180, h: 70, label: 'namespace dev', delay: 0}
    - {type: box, x: 390, y: 105, w: 180, h: 70, label: 'namespace production', tone: accent, delay: 0.2}
    - {type: seal, cx: 320, cy: 140, r: 30, broken: true, delay: 0.6}
    - {type: label, x: 320, y: 200, text: 'only as strong as the binding', tone: danger, delay: 0.9}
---
Put the last few screens together. A `namespace` groups objects. A `ServiceAccount`
gives a pod an identity. A `Role` lists permissions, and a binding hands them
out - to one namespace, or to all of them.

So when people say a namespace is not a security boundary, this is what they
mean. Nothing about the namespace itself stops a cross-namespace read. The only
thing standing there is whether some binding, somewhere, granted it.
