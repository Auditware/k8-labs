# rolebinding-escalate - self-granting a role via the bind verb (RBAC)

An RBAC privilege-escalation lab in the nested-cluster shape. The student lands
in a `dev`-namespace shell that can create RoleBindings in `production` and holds
the `bind` verb on one powerful ClusterRole. They bind that role to their own
identity and read the flag Secret.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/rolebinding-escalate.yaml`), applied
into the session's disposable nested k3s cluster. The foothold uses the shared
`k8labs-foothold` image - no per-module image. The flag is bait (see `_template`).

## The flaw, and the subtlety

Kubernetes has **privilege-escalation prevention**: to create a RoleBinding you
must already hold every permission the referenced role grants, or the API server
refuses it with `attempting to grant RBAC permissions not currently held`. So
`create rolebindings` alone is *not* enough to self-grant a role you lack.

The escape hatch is the `bind` verb (and its sibling `escalate`): holding `bind`
on a specific role exempts that role from the check. The planted flaw is exactly
this - `dev/sa-binder` is granted `create rolebindings` in `production` **and**
`bind` on the `secret-admin` ClusterRole. Together they let a powerless account
award itself `secret-admin` and read the flag. Removing the `bind` grant is
enough to make the escalation-prevention check reject the self-grant (verified in
testing).

## Solution (spoiler)

```sh
kubectl auth can-i --list -n production            # spot: create rolebindings + bind on secret-admin (bare --list checks dev, shows nothing)
kubectl create rolebinding pwn -n production \
  --clusterrole=secret-admin --serviceaccount=dev:sa-binder
kubectl get secret flag -n production -o jsonpath='{.data.flag}' | base64 -d
# k8labs{bind_verb_defeats_the_rbac_escalation_check}
```

## The fix

Never grant `bind`/`escalate` to general workloads - reserve them for the
controllers that provision access, and audit every use. A RoleBinding whose
subject is the same identity that created it is a strong signal. Treat any
account holding both create-rolebindings and `bind` on a strong role as already
holding that role.

## What a defender sees

The API server audit log records the RoleBinding `create`; the tell is that the
binding's subject matches the creating identity and the role reaches well beyond
that identity's own grants. Under Kata in prod, host-level Tetragon cannot see
inside the nested cluster, so this is an audit-log / in-cluster-telemetry
observation - the same design note as `ns-escape`.
