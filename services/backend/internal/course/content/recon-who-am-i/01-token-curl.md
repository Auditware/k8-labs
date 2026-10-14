---
kind: concept
ey: CONCEPT 1
eyt: The raw request
tag: technique
title: The token is a key, and curl is the lock it opens
aha: kubectl was only ever wrapping this - a bearer token in a header, over TLS, to one endpoint. Take the wrapper off and nothing is lost.
---
Everything kubectl did is one HTTPS call. The API server lives at
`https://kubernetes.default.svc`, your credential is the `token` file, and the
`ca.crt` beside it verifies the server. Put them together:

```
API=https://kubernetes.default.svc
T=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
CA=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt
curl -s --cacert $CA -H "Authorization: Bearer $T" $API/api/v1/namespaces
```

Run all four at once: `$ API=https://kubernetes.default.svc; T=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token); CA=/var/run/secrets/kubernetes.io/serviceaccount/ca.crt; curl -s --cacert $CA -H "Authorization: Bearer $T" $API/api/v1/namespaces`. This sets `API`, `T`, and `CA` in your shell, so every later step is just a `curl` that reuses them.

That `Authorization: Bearer` header is the whole authentication story. Every
request from here is a variation on this one line.
