---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: How do you best defend against a malicious mutating webhook?
opts:
  - t: Rotate every service-account token in the cluster
  - t: Tightly restrict who can create `mutatingwebhookconfigurations`, and alert on new ones
    correct: true
  - t: Add a NetworkPolicy in every namespace
  - t: Delete the injected sidecars as they appear
explain: Chasing injected sidecars is endless, and tokens or NetworkPolicies miss the point - the webhook is the persistence, not the pods. Treat `create` and update rights on webhook configurations as cluster-admin-equivalent, grant them to almost no one, and alert whenever a webhook configuration appears or changes. The config object is the thing to watch.
---
