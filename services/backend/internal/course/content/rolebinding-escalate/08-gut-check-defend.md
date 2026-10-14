---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: Which change actually closes this escalation?
opts:
  - t: Remove the account's `create rolebindings` grant, or its `bind` on the powerful role
    correct: true
  - t: Move the flag Secret to another namespace
  - t: Give `secret-admin` fewer rules
  - t: Rotate the service-account token
explain: The escalation needed two things together - the ability to create a binding, and `bind` on a role worth having. Remove either and it collapses - with no `bind`, the escalation-prevention check blocks the self-grant; with no create-binding grant, there is nothing to write. Moving the secret, trimming the role, or rotating the token leave the self-grant path intact.
---
