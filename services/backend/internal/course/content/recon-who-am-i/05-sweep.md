---
kind: concept
ey: CONCEPT 4
eyt: Sweeping the cluster
tag: technique
title: SelfSubjectRulesReview is can-i --list, over the wire
aha: One request per namespace turns a blind foothold into a map of exactly where your identity has reach.
---
You can enumerate the cluster's namespaces (the request from screen one,
`GET /api/v1/namespaces`), then ask the same rules question about each. Sweep
them:

```
for ns in $(curl -s --cacert $CA -H "Authorization: Bearer $T" \
  $API/api/v1/namespaces | jq -r '.items[].metadata.name'); do
  echo "== $ns =="
  curl -s --cacert $CA -H "Authorization: Bearer $T" \
    -H 'Content-Type: application/json' \
    -X POST $API/apis/authorization.k8s.io/v1/selfsubjectrulesreviews \
    -d "{\"apiVersion\":\"authorization.k8s.io/v1\",\"kind\":\"SelfSubjectRulesReview\",\"spec\":{\"namespace\":\"$ns\"}}" \
    | jq -c '.status.resourceRules[]'
done
```

Run the whole sweep: `$ for ns in $(curl -s --cacert $CA -H "Authorization: Bearer $T" $API/api/v1/namespaces | jq -r '.items[].metadata.name'); do echo "== $ns =="; curl -s --cacert $CA -H "Authorization: Bearer $T" -H 'Content-Type: application/json' -X POST $API/apis/authorization.k8s.io/v1/selfsubjectrulesreviews -d "{\"apiVersion\":\"authorization.k8s.io/v1\",\"kind\":\"SelfSubjectRulesReview\",\"spec\":{\"namespace\":\"$ns\"}}" | jq -c '.status.resourceRules[]'; done`. Watch the output roll past - one namespace answers differently from the rest.

One namespace answers differently from the rest: in `billing`, your rules
include `get` on `secrets`. That is a grant nobody ever told you that you had.
