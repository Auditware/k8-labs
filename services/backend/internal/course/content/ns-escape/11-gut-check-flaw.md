---
kind: quiz
ey: GUT-CHECK
eyt: Before you go
tag: trick question
q: '`kubectl auth can-i --list` in `dev` shows you can get secrets. What should you check next?'
opts:
  - t: Whether the same command says yes in another namespace
    correct: true
  - t: Whether the `Role` mentions production anywhere
  - t: Whether you can create a new `RoleBinding` for yourself
  - t: Whether the flag Secret has weak contents
explain: A permission list looks identical whether it came from a `RoleBinding` or a `ClusterRoleBinding` - what differs is where else it applies, so the fastest test is simply to ask about another namespace. Checking the role for a namespace is checking a field that does not exist, and creating your own binding would be a different attack requiring permissions you do not have.
---
