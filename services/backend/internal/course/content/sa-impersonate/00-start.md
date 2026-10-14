---
kind: concept
ey: START
eyt: Service Account Impersonation
tag: what you'll learn
title: You will read a secret you have no permission to read.
aha: The account you are handed can read nothing at all. That turns out not to matter, because it can borrow an account that can.
revealLabel: why this matters
diagram:
  caption: powerless yourself, powerful by proxy
  shapes:
    - {type: box, x: 55, y: 115, w: 180, h: 66, label: 'you', sub: 'can read nothing', delay: 0}
    - {type: wire, from: {x: 235, y: 148}, to: {x: 375, y: 148}, tone: danger, dashed: true, delay: 0.4}
    - {type: box, x: 375, y: 115, w: 205, h: 66, label: 'a borrowed identity', sub: 'can read everything', tone: danger, delay: 0.7}
    - {type: label, x: 305, y: 205, text: 'one verb bridges the two', delay: 1.0}
---
You have a shell in a cluster, running as an identity that has been granted
almost nothing. You cannot read the secret you are after. As yourself, you never
will.

Over the next few screens we will build up one permission the account *was*
given - and why that single permission quietly undoes every other limit placed
on it.
