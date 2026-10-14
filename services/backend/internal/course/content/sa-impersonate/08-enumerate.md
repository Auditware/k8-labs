---
kind: concept
ey: CONCEPT 6
eyt: Asking the cluster
tag: technique
title: Ask what you hold before you reach for it
aha: Most systems make you guess at your own permissions. This one reads them out loud to anyone holding the token.
diagram:
  caption: the API answers honestly about your own access
  shapes:
    - {type: box, x: 60, y: 118, w: 200, h: 64, label: 'your shell', sub: 'kubectl auth can-i', delay: 0}
    - {type: wire, from: {x: 260, y: 150}, to: {x: 370, y: 150}, delay: 0.4}
    - {type: box, x: 370, y: 118, w: 210, h: 64, label: 'API', sub: 'lists your verbs', tone: accent, delay: 0.6}
    - {type: label, x: 315, y: 205, text: 'read the verb, not just the resource', delay: 0.9}
---
Before doing anything, ask the cluster what your identity may do.
`$ kubectl auth can-i --list` prints every verb-and-resource pair your account
holds. Read it looking past the obvious - a missing `get secrets` is expected
here; the line that matters is the `impersonate` verb sitting on `users` and
`groups`.

You can also ask pointed questions: `$ kubectl auth can-i get secrets -n production`
answers the direct route (it will say no), and confirming you *can* impersonate
tells you the indirect route is open. The first move on any foothold is to read
your own grants.
