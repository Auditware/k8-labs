---
kind: quiz
ey: GUT-CHECK
eyt: Think like the defender
tag: trick question
q: You are reviewing this cluster. Which single change would have stopped the attack?
opts:
  - t: Move the flag Secret into a more restricted namespace
  - t: Remove the `impersonate` verb on `groups` from the account
    correct: true
  - t: Add a NetworkPolicy between dev and production
  - t: Rotate the `sa-imper` token
explain: Relocating the Secret or rotating the token changes nothing - `system:masters` reads across every namespace and the token is used legitimately. A NetworkPolicy governs traffic, not authorization. The escalation existed entirely because the account could impersonate a `group`; take that grant away and the borrowed superuser identity is simply unavailable.
---
