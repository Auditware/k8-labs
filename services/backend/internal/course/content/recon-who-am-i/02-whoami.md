---
kind: concept
ey: CONCEPT 2
eyt: Establishing identity
tag: technique
title: Ask the API who you are before you guess
aha: You do not have to assume which account you landed on. There is an endpoint whose only job is to tell you.
---
Before probing what you can do, settle who you are. A `SelfSubjectReview` is a
POST the API answers by echoing your own authenticated identity back at you:

```
curl -s --cacert $CA -H "Authorization: Bearer $T" \
  -H 'Content-Type: application/json' \
  -X POST $API/apis/authentication.k8s.io/v1/selfsubjectreviews \
  -d '{"apiVersion":"authentication.k8s.io/v1","kind":"SelfSubjectReview"}'
```

Run it: `$ curl -s --cacert $CA -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -X POST $API/apis/authentication.k8s.io/v1/selfsubjectreviews -d '{"apiVersion":"authentication.k8s.io/v1","kind":"SelfSubjectReview"}'`.

The reply names your user (`system:serviceaccount:dev:app`) and your groups.
Now you know exactly which identity every later request is made as - no
guessing.
