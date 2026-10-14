---
kind: quiz
ey: GUT-CHECK
eyt: Did it land?
tag: trick question
q: An account can `create rolebindings` and holds `bind` on a `secret-admin` `ClusterRole` it does not otherwise have. What can it do?
opts:
  - t: Nothing new - it still cannot grant a role it lacks
  - t: Grant `secret-admin` to itself, because `bind` exempts that role from the check
    correct: true
  - t: Only grant `secret-admin` to other accounts, not itself
  - t: Edit any role in the cluster
explain: The guard from two screens ago would normally block granting a role you lack. But `bind` on `secret-admin` is precisely the exemption for that role, and a binding's subject can be yourself. So the account writes a binding granting `secret-admin` to its own identity - and the check waves it through. Not every role, and not editing roles - just binding this one, which is enough.
---
