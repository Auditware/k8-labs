---
kind: concept
ey: CONCEPT 2
eyt: The wrapping
tag: technique
title: The password is under two layers of base64, not a lock
aha: base64 is not encryption. It is an encoding, reversible by anyone, and it is all that stands between a Secret's data and its plaintext.
diagram:
  caption: peel two encodings, no key required
  shapes:
    - {type: box, x: 35, y: 118, w: 150, h: 62, label: '.dockerconfigjson', sub: 'base64', delay: 0}
    - {type: wire, from: {x: 185, y: 149}, to: {x: 235, y: 149}, delay: 0.4}
    - {type: box, x: 235, y: 118, w: 130, h: 62, label: 'JSON', sub: 'auth field', delay: 0.6}
    - {type: wire, from: {x: 365, y: 149}, to: {x: 415, y: 149}, delay: 0.8}
    - {type: box, x: 415, y: 118, w: 175, h: 62, label: 'user:password', sub: 'base64', tone: accent, delay: 1.0}
---
Reading the credential is a matter of peeling encodings, none of which is a
secret in itself. Every Secret's `data` values are base64, so the
`.dockerconfigjson` field decodes to plain JSON. Inside that JSON, each registry
entry has an `auth` field - and `auth` is *itself* base64, of the literal string
`username:password`.

So there are two base64 layers and a JSON parse between you and the password, and
not one lock among them. `base64 -d` twice, split on the colon, and you are
holding the registry account. The obscurity is real - it does not look like a
password at a glance - but obscurity is not protection.
