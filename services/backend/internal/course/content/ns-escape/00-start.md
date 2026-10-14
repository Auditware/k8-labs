---
kind: concept
ey: START
eyt: Namespace Escape
tag: what you'll learn
title: A namespace keeps things tidy. It does not keep things safe.
aha: The lab you are about to break is not exotic. It is one word different from the correct configuration, and that word is easy to type by accident.
revealLabel: why this matters
diagram:
  caption: one wrong word turns a fence into a suggestion
  shapes:
    - {type: box, x: 60, y: 110, w: 190, h: 66, label: 'dev', sub: 'where you start', delay: 0}
    - {type: wire, from: {x: 250, y: 143}, to: {x: 390, y: 143}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 390, y: 110, w: 190, h: 66, label: 'production', sub: 'where the secret is', tone: accent, delay: 0.7}
    - {type: label, x: 320, y: 128, text: 'should not be possible', tone: danger, delay: 1.0}
---
You have a shell in one part of a cluster. A secret you were never meant to see
lives in another part. Nothing is broken, no software has a bug, and no password
is weak.

Over the next few screens we will build up exactly enough of Kubernetes'
permission system to see the mistake, then you will go and use it.
