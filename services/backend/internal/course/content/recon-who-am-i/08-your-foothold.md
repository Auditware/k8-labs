---
kind: concept
ey: BRIEFING
eyt: Your foothold
chip: {label: recap, tone: accent}
title: Here is what you were handed
aha: Three questions, asked with nothing but a token and curl - who am I, what can I do, and where does that reach.
---
No kubectl, no exploit - just three questions to the API server. You confirmed
your identity with a `SelfSubjectReview`, enumerated your power namespace by
namespace with `SelfSubjectRulesReview`, and found a `get` on `secrets` in
`billing` that nothing on the pod advertised.

That grant is your way in. Read the `revenue-report` `Secret` back and capture
it on the next screen.
