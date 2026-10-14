---
kind: concept
ey: START
eyt: RoleBinding Self-Grant
tag: what you'll learn
title: You can hand out permissions. The question is whether you can hand them to yourself.
aha: There is a rule that says you cannot give away what you do not own. This lab is about the one grant that quietly suspends that rule.
revealLabel: why this matters
diagram:
  caption: the interesting arrow points back at you
  shapes:
    - {type: box, x: 55, y: 115, w: 175, h: 66, label: 'you', sub: 'can grant roles', delay: 0}
    - {type: box, x: 320, y: 115, w: 210, h: 66, label: 'a powerful role', sub: 'granted to yourself', tone: danger, delay: 0.6}
    - {type: wire, from: {x: 230, y: 138}, to: {x: 320, y: 138}, tone: danger, delay: 1.0}
    - {type: wire, from: {x: 320, y: 160}, to: {x: 230, y: 160}, tone: danger, delay: 1.4}
---
You have a shell in a cluster, and the ability to grant permissions to accounts.
You cannot read the secret you are after. But granting is exactly the power that,
pointed the right way, lets you grant yourself the access you lack.

Over the next few screens we will meet the safeguard that is supposed to stop
this - and the single permission, sitting in your own grants, that turns it off.
