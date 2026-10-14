---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: You register a `MutatingWebhookConfiguration` for pod creation. Which pods does it affect?
opts:
  - t: Only pods you create yourself
  - t: Only pods in your own namespace
  - t: Every pod admitted anywhere that matches its rules - now and in the future
    correct: true
  - t: No pods until an admin approves the webhook
explain: The instinct is that your webhook only touches your own pods. But admission runs for every create the `API server` handles, so a webhook applies to all matching pods, cluster-wide, including ones created long after you are gone. That reach - and its persistence - is exactly what makes it an attacker's favourite.
---
