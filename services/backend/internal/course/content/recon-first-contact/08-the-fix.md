---
kind: concept
ey: DEFEND
eyt: The fix
tag: blue team
title: Stop shipping secrets as environment variables
aha: The fix is boring on purpose - keep the secret a file the process opens, not a string every child inherits.
---
The flaw was one line: a `secretKeyRef` in the container's `env`. Mount the
`Secret` as a file instead (a projected volume the app reads on demand), and the
value stays out of `env`, out of `/proc/<pid>/environ`, and out of every child
process.

Better still, do not hand a long-lived database password to the workload at all
- issue short-lived credentials it fetches at runtime. Either way the rule is
the same: a secret in an environment variable is a secret you have already
leaked.
