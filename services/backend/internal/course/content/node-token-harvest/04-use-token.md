---
kind: concept
ey: CONCEPT 3
eyt: Using a stolen token
tag: the machinery
title: A token is a bearer credential - hold it and you are that account
aha: There is no second factor, no check on who is carrying it. The token does not ask if you are its rightful owner; it just proves whoever holds it is the account.
diagram:
  caption: the token alone authenticates you
  shapes:
    - {type: box, x: 55, y: 116, w: 185, h: 66, label: 'kubectl --token', sub: 'stolen token', tone: danger, delay: 0}
    - {type: wire, from: {x: 240, y: 149}, to: {x: 355, y: 149}, tone: danger, delay: 0.4}
    - {type: box, x: 355, y: 116, w: 200, h: 66, label: 'API as collector', sub: 'its full access', tone: danger, delay: 0.7}
---
A `ServiceAccount` `token` is a bearer credential. The API server trusts whoever
presents it - it does not verify that the presenter is the pod it was issued to.
So a token read off the node is not a clue to an account; it *is* the account.

`kubectl` lets you present one directly with the `--token` flag: `$ kubectl --token`
uses the token you give it instead of your own. Point it at a harvested token and every call is
authorised as that identity. All that remains is choosing the right token to
carry.
