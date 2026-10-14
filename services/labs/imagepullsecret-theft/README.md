# imagepullsecret-theft - a pull secret is a stored registry credential

The supply-chain lab, and the only one about registry credentials. No eBPF, no
escape - a plain RBAC over-grant lets the foothold read Secrets, and one of those
Secrets is a `kubernetes.io/dockerconfigjson` imagePullSecret: a real registry
username and password, lightly obscured behind base64. The student harvests it
and decodes it.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/imagepullsecret-theft.yaml`), applied
into the session's disposable nested k3s cluster. The foothold uses the shared
`k8labs-foothold` image. The credential and registry are fictional
(`registry.internal:5000`, `deploy-bot`); the flag is the password half of the
stored `auth`.

## The flaw

The `station` service account is bound (Role + RoleBinding in `lab`) with
`get`/`list` on Secrets - a common over-grant. The `regcred` Secret in the same
namespace is type `kubernetes.io/dockerconfigjson`; its `.dockerconfigjson` is
base64 of a JSON doc whose `auth` field is base64 of `deploy-bot:<password>`.

## Solution (spoiler)

```sh
kubectl get secrets                       # spot: regcred, type dockerconfigjson
kubectl get secret regcred \
  -o jsonpath='{.data.\.dockerconfigjson}' | base64 -d
# -> {"auths":{"registry.internal:5000":{"username":"deploy-bot","auth":"..."}}}
echo <auth-value> | base64 -d             # -> deploy-bot:k8labs{...}
```

The flag is `k8labs{registry_pull_secret_is_a_stored_credential}` (the password
after the colon).

## The fix

Two least-privilege moves. Keep `get secrets` off workload service accounts - a
pod does not read its own pull secret; the kubelet uses it by reference. And
issue the registry credential as a pull-only, per-repository token, so a stolen
one cannot push a poisoned image. Small identity, small credential.
