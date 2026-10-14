# Self-hosting K8 Labs

Two ways to run it: a **local dev cluster** on your laptop, and a **production deployment** on
Hetzner. The local path runs the full app and session-provisioning flow; the only thing it cannot do
is boot real microVMs (that needs nested virtualization, i.e. bare metal), so local lab sessions run
under `runc` instead of Kata/Firecracker. Everything else - the UI, the backend, the nested k3s, the
eBPF Defender path - works locally.

> The architecture and the exact local/prod differences, variable by variable, are in
> [architecture/environments.md](architecture/environments.md) and [architecture/](architecture/).
> This guide is the map; those are the territory.

---

## Prerequisites

- **Docker** (Desktop or Engine)
- **Go 1.22+** and **[Bun](https://bun.sh/)** (for building backend/frontend outside containers)
- **kubectl**, **helm**, **[k3d](https://k3d.io/)**, and **[Tilt](https://tilt.dev/)** (local cluster)
- For production: a **Hetzner** account, **Terraform**, and a domain you control

---

## Local development

### Fast path - docker-compose

API + Postgres only, no cluster. Best for auth, the catalog, and the course UI.

```bash
cp secrets/backend.env.local.example secrets/backend.env     # dummy values are fine
cp secrets/frontend.env.example       secrets/frontend.env
docker compose up                                            # backend + postgres, DEV_LOGIN=1
cd services/frontend && bun install && bun run dev           # http://localhost:7373
```

This path cannot provision lab sessions (no cluster). Use the dev login to sign in.

### Full path - Tilt + k3d

A real cluster with live session provisioning. Lab sessions run under `runc` (no microVM wall
locally), but the whole backend/UI/nested-k3s/eBPF path is exercised.

```bash
# 1. k3d cluster (Flannel + built-in NetworkPolicy off; Cilium replaces them)
k3d cluster create k8labs-local --registry-create k8labs-registry:0.0.0.0:5000 \
  --agents 1 --port "8080:80@loadbalancer" --port "8443:443@loadbalancer" \
  --k3s-arg '--flannel-backend=none@server:*' --k3s-arg '--disable-network-policy@server:*'

# 2. Dedicated labs node (mirrors the prod control-plane / labs split)
kubectl label node k3d-k8labs-local-agent-0 k8labs.io/role=labs
kubectl taint node -l k8labs.io/role=labs k8labs.io/dedicated=labs:NoSchedule --overwrite

# 3. Cluster prerequisites (installed directly, not via ArgoCD)
helm install cilium cilium/cilium -n kube-system \
  --set kubeProxyReplacement=false --set ipam.mode=kubernetes \
  --set cni.exclusive=false --set operator.replicas=1
helm install cert-manager jetstack/cert-manager -n cert-manager --create-namespace \
  --set crds.enabled=true

# 4. Hot-reload dev loop
tilt up                                   # Tilt dashboard on :10350
cd services/frontend && bun install && bun run dev
```

The app serves at `https://localhost:8443` (cert-manager issues a self-signed cert; trust the local
root CA or use `curl --cacert`). The Tiltfile sets every local-vs-prod seam via Helm `--set`
(empty `labsRuntimeClass` so sessions use `runc`, `config.devLogin=1`, and so on).

Helper scripts in [`scripts/`](../scripts/):
- `dev-testuser.sh` - create a local non-admin test user + API key (guards on the `k3d-k8labs-local`
  context).
- `local-status.sh` - check Tilt is alive and the running backend matches your HEAD.

See [architecture/environments.md](architecture/environments.md) for the optional pieces (ArgoCD,
Vault, Tetragon, Loki/Grafana/Alloy) and the exact version pins.

---

## Production on Hetzner

### Why bare metal

Lab sessions run each attack inside a **Kata/Firecracker microVM** - a real second kernel is the
isolation boundary that contains a container or host escape. That needs **nested virtualization
(KVM)**, which Hetzner Cloud VMs do not expose but Hetzner **dedicated (bare-metal)** servers do. So
the labs node is always a dedicated box. The control plane and app can live on either.

### Two footprints

| | Budget (single box) | Reference (two nodes) |
|---|---|---|
| **Hardware** | 1x Hetzner dedicated (e.g. AX41, Ryzen, 64 GB, KVM) | 1x cloud VM (control plane + app) + 1x dedicated (labs) |
| **Rough cost** | ~$45-55 / month | ~$150-175 / month |
| **Trade-off** | Everything on one host; simplest and cheapest | App/control-plane isolated from the lab blast radius; room to scale lab concurrency |

The budget single-box option runs the control plane, the app, and the microVM lab node on one
dedicated server. It is the cheapest way to stand the whole thing up and comfortably fits a ~$70/month
budget. The public reference deployment proved this footprint works; scale out to the two-node layout
only once lab concurrency earns it.

### 1. Provision infrastructure

Terraform for Hetzner Cloud lives in [`infra/cloud/`](../infra/cloud/). Supply your own values (there
are **no** real credentials, IPs, or keys in the repo):

```bash
cp secrets/hcloud.env.example secrets/hcloud.env            # add your HCLOUD_TOKEN
export TF_VAR_ssh_public_key="ssh-ed25519 AAAA... you@host" # your own key
cd infra/cloud && terraform init && terraform apply
```

Set `admin_cidr` to your own IP range so only you can reach SSH and the admin surfaces. The dedicated
(bare-metal) labs server is ordered through Hetzner Robot and joined to the cluster; see
[`infra/cluster/`](../infra/cluster/).

### 2. Bring up the cluster

The reference cluster is a 2-node **kubeadm** setup. The step-by-step (install, CNI, labels/taints,
the `kata-fc` RuntimeClass, and a boot check that a test pod really lands in a microVM) is in
[`infra/cluster/kubeadm-2node.md`](../infra/cluster/kubeadm-2node.md) and
[`infra/cluster/bootstrap.sh`](../infra/cluster/bootstrap.sh). RuntimeClass definitions are under
[`infra/runtimeclass/`](../infra/runtimeclass/).

### 3. Deploy the app (GitOps)

Deployment is GitOps via **ArgoCD**. The app-of-apps in [`argocd/`](../argocd/) points at the Helm
charts in [`apps/`](../apps/). Set your domain in `apps/frontend/values.yaml` and
`apps/backend/values.yaml` (replace the `k8labs.example.com` placeholder), point your DNS at the
ingress, and cert-manager + Let's Encrypt will issue the real certificate.

### 4. Secrets

Nothing secret ships in this repo. In the cluster, inject secrets at runtime from your secret manager
of choice - the reference deployment uses **HashiCorp Vault** (Vault Agent Injector), but SOPS,
Sealed Secrets, or plain Kubernetes Secrets work the same way. See [`secrets/README.md`](../secrets/README.md)
for the full list (DB password, JWT signing key, OAuth client secrets, registry pull credentials) and
`secrets/backend.env.prod.example` for the variable shapes.

### 5. OAuth

Create a Google and/or GitHub OAuth app and register `https://<your-domain>/login` as the redirect
URI. Put the client secrets in your secret manager; the client IDs (which are not secret) go in the
frontend build args. With neither configured, the login buttons simply render disabled.

---

## Teardown

```bash
# local
k3d cluster delete k8labs-local
# cloud
cd infra/cloud && terraform destroy
```
