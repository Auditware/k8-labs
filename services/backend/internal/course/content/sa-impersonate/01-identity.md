---
kind: concept
ey: CONCEPT 1
eyt: Who is asking
tag: foundation
title: Every request to the cluster carries an identity
aha: The API server never sees "a program". It sees a name and a list of groups, and it decides what to allow from those two things alone.
diagram:
  caption: a request is a name plus its groups
  shapes:
    - {type: box, x: 60, y: 115, w: 170, h: 66, label: 'request', sub: 'get secret', delay: 0}
    - {type: wire, from: {x: 230, y: 148}, to: {x: 350, y: 148}, delay: 0.4}
    - {type: box, x: 350, y: 100, w: 220, h: 96, label: 'API server', sub: 'who are you? which groups?', tone: accent, delay: 0.7}
---
When anything talks to the Kubernetes API, the request arrives tagged with an
`identity`: a user or account name, plus a set of `group` memberships. The API
server authorises the request against those - what may this name, in these
groups, do here.

Nothing else about the caller matters to the decision. Not what program it is,
not how it got its credentials - only the name and the groups it presents. That
makes an identity something worth stealing, and, as you will see, something the
cluster is sometimes willing to *lend*.
