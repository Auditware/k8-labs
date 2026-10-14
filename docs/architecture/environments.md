# Local vs prod: what actually differs

Goal: the same code and the same container images run in both places. Only
**where a value comes from** changes - never the variable name, never the
code path that reads it. This is the map of every place that's true, and
the two places it's deliberately *not* true (flagged below, because pretending
otherwise is how drift bugs happen).

## The split: secret vs config

Two different axes, don't conflate them:

- **Secret vs not** - decides whether Vault or a plain ConfigMap owns it in prod.
- **Local vs prod** - decides *where the value physically comes from* right now.

| Variable | Secret? | Local source | Prod source |
|---|---|---|---|
| `PORT` | No | `secrets/backend.env` | Helm `values.yaml` (ConfigMap) |
| `LOG_LEVEL` | No | `secrets/backend.env` | Helm `values.yaml` (ConfigMap) |
| `DATABASE_HOST` | No | `secrets/backend.env` (`postgres`, the compose service name) | ConfigMap (`postgres.<namespace>.svc.cluster.local`, the k8s Service DNS name) |
| `DATABASE_PORT` / `DATABASE_NAME` / `DATABASE_SSLMODE` | No | `secrets/backend.env` | ConfigMap |
| `DATABASE_USER` / `DATABASE_PASSWORD` | **Yes** | `secrets/backend.env` (git-secret encrypted at rest) | **Vault**, KV engine, injected by Agent Injector |
| `JWT_SIGNING_KEY` | **Yes** | `secrets/backend.env` (dummy value - never the real prod key) | **Vault** |
| `HCLOUD_TOKEN` | **Yes** | `secrets/hcloud.env` (git-secret) - read by **Terraform** (`infra/cloud/`), never by the app | same file, or a CI secret if apply runs in CI. Not an app runtime var - the app never sees it. See "Infra provisioning config" below |
| `CORS_ORIGIN` | No | `secrets/backend.env` | ConfigMap |
| `GOOGLE_CLIENT_ID` / `GITHUB_CLIENT_ID` | **Yes**\* | `secrets/backend.env` | **Vault** |
| `GOOGLE_CLIENT_SECRET` / `GITHUB_CLIENT_SECRET` | **Yes** | `secrets/backend.env` | **Vault** |
| `VITE_GOOGLE_CLIENT_ID` / `VITE_GITHUB_CLIENT_ID` | No\* | `secrets/frontend.env`, loaded host-side by `vite.config.ts` (bun run dev/build) | Docker build arg - see below, this is the flagged exception |

\* The client IDs aren't confidential (they're visible in the browser's own redirect URL to Google/GitHub regardless) - grouped with the secrets above only because that's where our OAuth convention put them and because `GOOGLE_CLIENT_ID`/`GITHUB_CLIENT_ID` sit in the same Vault path as their actual secrets. `VITE_GOOGLE_CLIENT_ID`/`VITE_GITHUB_CLIENT_ID` are the frontend's own copies of the same non-secret values.

Rule of thumb: if it's in the "Secret?" = Yes column, **the local value must never be the real prod value** - not even briefly, not even to "test something quickly." `secrets/backend.env` is gitignored specifically so this is safe to get wrong once without it leaking, but treat it as a hard rule anyway.

## Infra provisioning config (not app runtime env)

The table above is *application* env - what a running pod reads. Provisioning
config (used by Terraform/kubeadm to build the cluster, never by the app) lives
elsewhere and follows the same secret-vs-not split. Listed here so this doc
stays the single greppable index (there is deliberately **no** central `envs/`
folder - config is colocated with its tool, secret-vs-not decides encryption;
see `secrets/README.md` for that split).

| Thing | Secret? | Where | Consumed by |
|---|---|---|---|
| `HCLOUD_TOKEN` | **Yes** | `secrets/hcloud.env` (git-secret) | Terraform (`infra/cloud/`) - provider reads it from env: `source secrets/hcloud.env && terraform apply` |
| AX41 SSH **private** key | **Yes** | `secrets/k8labs-robot` (git-secret) + your password manager | you, to SSH the labs node; CAPH would read it (Path A only) |
| AX41 SSH **public** key | No | inline in `infra/cloud/variables.tf` (`ssh_public_key`) - public by design, safe to commit | Terraform (uploads it to the Cloud node) |
| `admin_cidr`, `labs_node_cidr`, `server_type`, `location` | No | `infra/cloud/variables.tf` defaults + `infra/cloud/terraform.tfvars` (gitignored) | Terraform |
| Terraform state | **Yes** (may hold sensitive values) | `infra/cloud/*.tfstate` - **gitignored**, local-only for v1 | Terraform |

Cluster-scheduling config that isn't an env var but differs local↔prod, for
completeness:

- **Labs node taint/toleration**: both envs taint the labs node
  `k8labs.io/dedicated=labs:NoSchedule` so only lab pods land there; lab pods
  carry the matching toleration (`internal/labs` `labsTolerations`). Prod taints
  the bare-metal node in its kubeadm join (`infra/cluster/kubeadm-2node.md`);
  local applies the same taint right after labelling the k3d agent (see the
  cluster-create block below). Keeping local tainted too means the backend and
  other platform pods schedule onto the control-plane node in both envs, not
  the labs node - no local↔prod placement drift.
- **`labsRuntimeClass`**: `""` (runc) local, `kata-fc` (microVM) prod - the one
  isolation seam. See `apps/backend/values.yaml`.

## The two places this stops being "just swap the source"

**1. `DATABASE_HOST` changes hostname format, not just the source.** Locally it's a docker-compose service name (`postgres`); in prod it's a full k8s Service DNS name (`postgres.<namespace>.svc.cluster.local`). The backend doesn't care - it's still just a string in `DATABASE_HOST` - but if you ever hardcode the local value anywhere instead of reading the env var, it will silently work locally and silently fail in prod. This is exactly the kind of drift this doc exists to catch.

**2. The frontend's `VITE_*` vars are baked in at *build* time, not read at runtime.** Unlike the backend (`os.Getenv` - same image, any environment, value read fresh at container start), Vite inlines `VITE_*` values into the static JS bundle when `vite build` runs. A different value per environment would mean **a different image per environment**, not one image promoted through environments - which breaks the whole point of building once and deploying the same artifact.

The fix already in place: the frontend calls the backend via a **relative** `/api` path everywhere, never an absolute URL.
- Local: Vite's dev server proxy (`vite.config.ts`) forwards `/api` → `http://localhost:8080`.
- Prod: Traefik routes `/api/*` → the Backend Service, everything else → the Frontend Service (see `apps/frontend/.gitkeep` and `apps/backend/.gitkeep`).

Same relative path, same bundle, works in both - `secrets/frontend.env.example` is nearly empty because of this. If a future `VITE_*` var genuinely can't be made relative, it goes in this table with a note that it requires a prod-specific build - don't let that become the default.

**3. `VITE_GOOGLE_CLIENT_ID`/`VITE_GITHUB_CLIENT_ID` are exactly that exception.** They can't be relative - there's no "backend routes it" trick for a value the browser needs before it ever talks to the backend at all (building the OAuth authorize URL). Handled two different ways depending on how the frontend is actually running, because the Dockerfile still must never read `secrets/` directly (see `secrets/README.md`):
- `bun run dev`/`bun run build` (host-side, not Docker): `vite.config.ts` reads `secrets/frontend.env` itself and merges it into `process.env` before Vite starts - same idea as docker-compose's `env_file` for the backend, just Vite-side since there's no equivalent native mechanism.
- `docker build`: the Dockerfile declares `ARG VITE_GOOGLE_CLIENT_ID`/`ARG VITE_GITHUB_CLIENT_ID` and promotes them to `ENV` before `bun run build` runs inside the image - generic build args, agnostic to where the caller sourced the values from (locally: `secrets/frontend.env`; CI: its own secret store, still TODO to actually wire into `.github/workflows/ci.yml`).

Today both environments use the *same* Google/GitHub OAuth app (see `secrets/backend.env`'s comment on why that's safe for Google, not for GitHub), so this doesn't yet force a genuinely different image per environment - but the mechanism exists for the day it does.

## Running it end to end

Two ways to run the backend locally - pick based on what you're touching:

**docker-compose** - postgres + backend as plain processes, starts in seconds, no Kubernetes underneath. Fast for iterating on backend/auth/catalog logic. Session provisioning (starting a lab) can't work here structurally - there's no cluster for client-go to talk to, see `internal/labs`'s in-cluster-config check.

```
cp secrets/backend.env.local.example secrets/backend.env
# fill in real values only where the template says to - dummy values are fine
# for JWT_SIGNING_KEY/DATABASE_PASSWORD locally, see the rule above.
# The local postgres container reads the same file (its own POSTGRES_*
# names, see the comment in backend.env.local.example) - nothing else to copy.

docker compose up          # postgres + backend
cd services/frontend && bun install && bun run dev   # frontend, native for HMR
```

**Tilt against k3d** - the real cluster, real session provisioning, real hot-reload (see root `Tiltfile`). Slower to start (builds+pushes images, installs Helm charts), needed for anything touching `internal/labs` or the terminal proxy.

The cluster is created with Flannel and k3s's built-in NetworkPolicy controller disabled - Cilium replaces both, so NetworkPolicy is actually enforceable locally instead of being a silent no-op (Flannel doesn't enforce NetworkPolicy at all). `kubeProxyReplacement=false` because k3s already runs its own embedded kube-proxy; `cilium install`'s CLI auto-detection gets this wrong for k3d specifically (it lifts `0.0.0.0:<port>` straight out of the local kubeconfig as `KUBERNETES_SERVICE_HOST`, which only resolves from the host, not from inside a pod) - installing the chart directly with explicit `--set` values sidesteps that.

```
k3d cluster create k8labs-local --registry-create k8labs-registry:0.0.0.0:5000 \
  --agents 1 \
  --port "8080:80@loadbalancer" --port "8443:443@loadbalancer" \
  --k3s-arg '--flannel-backend=none@server:*' --k3s-arg '--disable-network-policy@server:*'

# Dedicated labs node - mirrors prod's node-level split between the
# control plane and the Kata-fc labs node group (see README.md's component
# diagram) instead of leaving that separation purely namespace-deep.
# Tilt's backend Helm values set config.labsNodeSelector to this same
# key=value pair (see Tiltfile) so lab pods actually land here.
kubectl label node k3d-k8labs-local-agent-0 k8labs.io/role=labs

# Match prod: taint that same labs node so platform pods (backend, etc.) stay
# off it and only lab pods - which carry the matching toleration (`internal/
# labs` labsTolerations) - schedule there. Keyed off the label just set above
# rather than the node name, so it survives k3d's node-name format (…-agent-0
# vs …-agent-0-0). Without it the untainted local agent lets the scheduler
# drift platform pods onto the labs node, unlike prod.
kubectl taint node -l k8labs.io/role=labs k8labs.io/dedicated=labs:NoSchedule --overwrite

helm repo add cilium https://helm.cilium.io/ && helm repo update cilium
helm install cilium cilium/cilium --version 1.16.5 --namespace kube-system \
  --set kubeProxyReplacement=false --set ipam.mode=kubernetes --set cni.exclusive=false --set operator.replicas=1
cilium status --wait        # confirm it's healthy before continuing

helm repo add jetstack https://charts.jetstack.io --force-update && helm repo update jetstack
helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace \
  --version v1.16.2 --set crds.enabled=true
# apps/cert-manager (deployed by Tilt below) sets up the self-signed root CA
# -> ca-issuer ClusterIssuer chain that apps/backend's Ingress actually uses.

tilt up                     # cert-manager issuers + postgres + labs RBAC + backend, dashboard at :10350
cd services/frontend && bun install && bun run dev   # same as above, still native
```

Once `tilt up` has issued the backend's cert (`kubectl get certificate backend-backend-tls` → `READY: true`), `https://localhost:8443` works with a real, cert-manager-issued cert - just not one your browser/curl trusts yet, since the root is one we minted, not a public CA. Trust it explicitly to browse without warnings:

```
kubectl get secret -n cert-manager ca-root-secret -o jsonpath='{.data.ca\.crt}' | base64 -d > /tmp/k8labs-local-ca.crt
curl --cacert /tmp/k8labs-local-ca.crt https://localhost:8443/api/v1/modules   # or add it to your OS/browser trust store
```

**ArgoCD** - same treatment as Cilium/cert-manager: a cluster prerequisite installed directly, not something ArgoCD manages itself (it can't manage its own first install before it exists). `argocd/` (repo root, separate from `apps/`) holds the `AppProject` + `Application` manifests pointing at `apps/**` - ArgoCD's own bootstrap wiring, applied once via `kubectl`, not itself GitOps-managed.

```
helm repo add argo https://argoproj.github.io/argo-helm && helm repo update argo
helm install argocd argo/argo-cd --namespace argocd --create-namespace --version 7.7.11 \
  --set 'configs.params.server\.insecure=true' \
  --set 'configs.params.server\.rootpath=/admin/argocd'

kubectl apply -f argocd/project.yaml
kubectl apply -f argocd/applications/
```

`server.insecure=true` because our own admin proxy (below) speaks plain HTTP to `argocd-server` in-cluster - TLS is already terminated at our own ingress, a second hop of it here would just add complexity with no real security benefit. `server.rootpath` is what makes ArgoCD generate all its own links/asset paths under `/admin/argocd/` instead of `/` - same idea as Grafana's `serve_from_sub_path` above.

**Vault** - real Integrated Storage (Raft), not `-dev` mode, so it genuinely needs init + unseal like a real environment would, and re-seals whenever the pod restarts.

```
helm repo add hashicorp https://helm.releases.hashicorp.com && helm repo update hashicorp
helm install vault hashicorp/vault --namespace vault --create-namespace \
  --set server.dataStorage.enabled=true --set server.dataStorage.size=1Gi \
  --set server.ha.enabled=true --set server.ha.replicas=1 \
  --set server.ha.raft.enabled=true --set server.ha.raft.setNodeId=true \
  --set injector.enabled=true

kubectl exec -n vault vault-0 -- vault operator init -key-shares=5 -key-threshold=3 -format=json > secrets/vault-unseal-keys.json
# gitignored (see .gitignore's *unseal*key* pattern) - never commit this. The
# root token lives in this JSON's root_token field; no separate token file.

for key in $(python3 -c "import json; d=json.load(open('secrets/vault-unseal-keys.json')); print('\n'.join(d['unseal_keys_b64'][:3]))"); do
  kubectl exec -n vault vault-0 -- vault operator unseal "$key"
done

ROOT_TOKEN=$(python3 -c "import json; print(json.load(open('secrets/vault-unseal-keys.json'))['root_token'])")
kubectl exec -n vault vault-0 -- sh -c "VAULT_TOKEN=$ROOT_TOKEN vault secrets enable -path=secret kv-v2"
kubectl exec -n vault vault-0 -- sh -c "VAULT_TOKEN=$ROOT_TOKEN vault auth enable kubernetes"
kubectl exec -n vault vault-0 -- sh -c "VAULT_TOKEN=$ROOT_TOKEN vault write auth/kubernetes/config kubernetes_host=\"https://\$KUBERNETES_SERVICE_HOST:\$KUBERNETES_SERVICE_PORT\""
```

Then the backend's own policy/role, and the actual secret values (sourced from `secrets/backend.env` locally):

```
kubectl exec -i -n vault vault-0 -- sh -c "VAULT_TOKEN=$ROOT_TOKEN vault policy write backend -" <<'EOF'
path "secret/data/backend" {
  capabilities = ["read"]
}
EOF

kubectl exec -n vault vault-0 -- sh -c "VAULT_TOKEN=$ROOT_TOKEN vault write auth/kubernetes/role/backend \
  bound_service_account_names=backend-backend bound_service_account_namespaces=default \
  policies=backend ttl=1h"

kubectl exec -n vault vault-0 -- sh -c "VAULT_TOKEN=$ROOT_TOKEN vault kv put secret/backend \
  database_user='...' database_password='...' jwt_signing_key='...' \
  google_client_id='...' google_client_secret='...' github_client_id='...' github_client_secret='...'"
```

`apps/backend`'s `vault.enabled` defaults to `true` (matching real environments) - Tilt uses that default too instead of the old `localSecret.*` fallback.

**Sign-in is Google/GitHub OAuth only, locally as well as in prod.** The seeded `admin/admin` password account and the whole `adminSeed` toggle were removed so local dev exercises the same auth path as prod rather than a shortcut that hides prod-only failures. Two consequences for a local cluster:

- Local Vault needs real `google_client_id`/`google_client_secret` at `secret/backend`, or the code exchange fails with `Could not determine client ID from request`. Vite reads the frontend's `VITE_*` values straight out of `secrets/frontend.env` (see `services/frontend/vite.config.ts`), so only the backend half needs Vault.
- Admin rights come from the `ADMIN_EMAILS` allowlist (`internal/accounts/grants.go`), read from Vault's `admin_emails` key. Without it you can sign in locally but land as `role='user'` with no admin panel and no lab grants. Add your own address to the `admin_emails` key in local Vault to get the admin panel and lab grants.

### Testing the non-admin path locally

Verifying the authorization boundary needs a second, non-admin account, and OAuth-only sign-in means you can't just make one up. `scripts/dev-testuser.sh` creates one directly in the local database along with an API key:

```
scripts/dev-testuser.sh --grant ns-escape   # create + print an API key
scripts/dev-testuser.sh --delete                # remove it again
```

It prints ready-to-run curl commands for the three checks worth making: the account authenticates, sees only its granted labs, and gets `403` on `/admin/*`. The API spec it receives from `/swagger/doc.json` has the admin routes stripped (see `internal/api/apidocs.go`).

The account **cannot sign in through the browser** - it has no OAuth identity and password login no longer exists. It's usable through its API key, and as a row in the admin Users page for exercising grant/revoke/block/delete.

This is deliberately a script and not startup seeding. The app used to seed an admin account at boot (`adminSeed`), and the row it created outlived the toggle: prod still carried that account long after the Secret was deleted. Seeding from inside the binary makes rows that survive the flag that made them. A script can't, because it isn't in the binary - and it refuses to run against any kubectl context other than `k3d-k8labs-local`.

The repo is private, so ArgoCD needs its own read credential - a fine-grained GitHub PAT (Contents: Read-only, scoped to this repo), stored encrypted at `secrets/argocd.env.secret` like everything else under `secrets/` (`git secret reveal` to get `secrets/argocd.env` back, same as `backend.env`/`frontend.env`):

```
GITHUB_PAT=$(grep GITHUB_PAT secrets/argocd.env | cut -d= -f2-)
kubectl create secret generic k8labs-repo-creds -n argocd \
  --from-literal=type=git \
  --from-literal=url=https://github.com/Auditware/k8-labs.git \
  --from-literal=username=x-access-token \
  --from-literal=password="$GITHUB_PAT" \
  --dry-run=client -o yaml | kubectl label --local -f - argocd.argoproj.io/secret-type=repository -o yaml | kubectl apply -f -
```

Locally these `Application`s want `syncPolicy: {}` (manual sync only) - Tilt is what's actually managing these same resources for local dev, with overrides (dev image tags, `ingress.tls.enabled=true`) that aren't in the charts' checked-in defaults. Auto-sync would fight Tilt over the same Deployments every reconcile loop. Expect `OutOfSync` on `kubectl get applications -n argocd` for exactly that reason - it's ArgoCD correctly detecting real drift, not a bug to chase down. Revisit once this cluster's local-dev-via-Tilt and GitOps-via-ArgoCD roles are actually reconciled.

> **Warning:** the checked-in `argocd/applications/*.yaml` now carry `syncPolicy.automated.selfHeal` for prod. After `kubectl apply -f argocd/applications/` against a local cluster you must strip it, or local ArgoCD will auto-sync git over Tilt's dev state and fight Tilt:

```
for a in backend frontend postgres labs cert-manager-issuers tetragon-policy; do
  kubectl -n argocd patch app $a --type json \
    -p '[{"op":"remove","path":"/spec/syncPolicy/automated"}]'
done
```

**Tetragon** - the EDR sensor, and a deliberate swap from Falco: Falco needs kernel module compilation or a debugfs mount, neither of which Docker Desktop's macOS VM exposes (confirmed via its own GitHub issues - k3d + Falco is a known, unresolved failure). Tetragon uses BTF/CO-RE instead - the same mechanism Cilium's own eBPF datapath already relies on here - so it needs neither. Same cluster-prerequisite treatment as Cilium/cert-manager/ArgoCD/Vault:

```
helm install tetragon cilium/tetragon --namespace kube-system --version 1.7.0
```

`apps/tetragon/` holds the actual detection policy: a `TracingPolicyNamespaced` on the session-runner namespace hooking the kernel's `commit_creds()`, which fires on every process credential change - including the automatic euid elevation `execve()` does for a SUID-root binary. This is deliberately not Tetragon's own `privileges-raise.yaml` example policy, which only catches an explicit `setuid()` call - the interesting privilege jumps happen inside the kernel's exec-time handling of the SUID bit itself.

**Coverage differs by runtime, and this matters.** Locally (k3d/runc) the nested cluster shares the host kernel, so the policy observes processes inside it. In prod (Kata/Firecracker) the microVM kernel boundary means host eBPF **cannot** see inside the nested cluster - that is the containment working as designed, but it also means there is no in-microVM EDR coverage of what a student actually does. What remains visible from `labs-runners` is the runner's host-side surface, which makes an unexpected `commit_creds` event there a possible **breakout** indicator rather than routine lab activity. See `apps/tetragon/templates/process-creds-changed.yaml`.

*Historical note (2026-07 and earlier): this policy was originally scoped to the `labs`/`labs-baseline` namespaces and was verified against the real `suid-privesc-101` exploit - the captured event's `process_credentials_arg` showed `uid: 10001` (labuser, unchanged) alongside `euid: 0` (elevated), and that mismatch was the "uid transition on exec" signal. That module and those namespaces were removed (see lab-session-rewrite.md, F1); the verification stands as a record of the hook working, not of current coverage.*

**Loki/Grafana/Alloy** - self-hosted logging, single-node scale (`singleBinary`/no clustering, no separate object storage - filesystem, matching this cluster's actual size). Same cluster-prerequisite treatment; unlike cert-manager/Tetragon there's no companion CRD-config layer needed under `apps/**`, since it's purely Helm-values configuration on top of the upstream charts:

```
helm repo add grafana https://grafana.github.io/helm-charts && helm repo update grafana

helm install loki grafana/loki --namespace observability --create-namespace --version 7.0.0 \
  --set deploymentMode=SingleBinary --set loki.auth_enabled=false \
  --set loki.commonConfig.replication_factor=1 --set loki.storage.type=filesystem \
  --set loki.useTestSchema=true --set singleBinary.replicas=1 \
  --set read.replicas=0 --set write.replicas=0 --set backend.replicas=0 \
  --set gateway.enabled=false --set test.enabled=false --set lokiCanary.enabled=false \
  --set chunksCache.enabled=false --set resultsCache.enabled=false

helm install grafana grafana/grafana --namespace observability -f docs/architecture/grafana-values.yaml

helm install alloy grafana/alloy --namespace observability --version 1.10.1 \
  --set-file alloy.configMap.content=docs/architecture/alloy-config.river
```

Verified end-to-end, not just installed: ran a real lab exploit, then queried Loki directly (`{namespace="kube-system", container="export-stdout"} |= "backup-tool"`) and got back Tetragon's actual `process_exec`/`process_exit` events for it, correctly labeled with `pod_labels.session`/`module`. Plain application logs flow the same way - `{namespace="default", container="backend"}` returns the backend's real structured JSON logs. One pipeline, both signals, no Tetragon-specific wiring needed - it's just another container's stdout to `loki.source.kubernetes` (see `alloy-config.river`'s own comment on why). *That run predates the nested rewrite and used the since-removed `suid-privesc-101` module; the log pipeline itself is unchanged, but see the coverage caveat above for what Tetragon can still observe under Kata.*

**Admin dashboard: cluster health + reverse-proxied links.** `GET /api/v1/admin/health` (`internal/health`) reports pod status across every namespace above, gated by `requireAdmin` (role checked fresh from the DB, not trusted from the JWT). `/admin/grafana/`, `/admin/argocd/`, `/ui/`, `/v1/` (`internal/api/adminproxy.go`) reverse-proxy each tool's own UI, same `requireAdmin` gate - and since these serve real HTML/CSS/JS rather than JSON, both `apps/backend`'s Ingress and the frontend's own `vite.config.ts` dev proxy route them to the backend the same way `/api/*` already does.

Grafana and ArgoCD both work as real one-click links under `/admin/*` - both explicitly support being served under a path prefix (`serve_from_sub_path`/`root_url` above, `server.rootpath` above), so every asset actually resolves under `/admin/grafana/`/`/admin/argocd/`, verified in a real browser, not just curl.

**Vault's UI can't be mounted under `/admin/vault` the same way** - it hardcodes absolute `/ui/assets/...` and `/v1/...` paths with no subpath config option at all (confirmed against the still-open `hashicorp/vault#18668`; a body-rewriting proxy workaround exists in the wild but isn't worth the fragility - breaks on every asset-hash-changing Vault upgrade). Instead of nesting it under `/admin`, it's mounted at the literal root paths its own JS already requests - `/ui/` and `/v1/` - confirmed via a live network capture of every request Vault's UI makes (nothing outside those two prefixes). Same `requireAdmin` gate as Grafana/ArgoCD, just matched on Vault's own paths instead of an `/admin/*` prefix. Two tradeoffs worth naming: this reserves `/ui` and `/v1` on the app's own domain permanently (nothing else here can ever use them), and `/v1/` is Vault's actual API, not just UI-support calls - so this proxies Vault's full read/write surface, not a read-only dashboard like Grafana. Vault's own token auth still gates every request behind `requireAdmin`, same defense-in-depth as everywhere else.

ArgoCD, Grafana, and Vault all keep their own separate logins underneath `requireAdmin`'s gate (`admin`/the chart-generated password for ArgoCD, `admin`/`admin` for Grafana locally, Vault's own root token) - no SSO integration exists yet, so reaching the proxy only proves you're an admin *here*, not automatically inside the proxied tool itself.

**Retrieving each one, locally:**
- **Grafana** - `admin` / `admin`, hardcoded in `docs/architecture/grafana-values.yaml` (documented there as never a real credential).
- **ArgoCD** - `admin` / a chart-generated password, live in a k8s Secret, not a file: `kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath='{.data.password}' | base64 -d`. Deliberately not copied into a file here - it can be rotated (`argocd account update-password`), and a static copy would just go stale and mislead whoever finds it later. The Secret itself is the only source of truth.
- **Vault** - root token is the `root_token` field in `secrets/vault-unseal-keys.json` (local) or `secrets/vault-unseal-keys.prod.json` (prod, git-secret'd). Read it: `python3 -c "import json;print(json.load(open('secrets/vault-unseal-keys.json'))['root_token'])"`. No separate token file.
- **Swagger/API docs** - admin-gated at `/swagger/` (requireAdmin + ingress IP-allowlist, same as the other panels); see `internal/api/routes.go`.

Both read the same `secrets/backend.env` - no separate config to maintain.

`secrets/backend.env.prod.example` is the prod-side counterpart - reference only, never copied or run (see `secrets/README.md`). Read it alongside the table above when it's time to actually write the Helm ConfigMap values and the Vault KV entries.

Deploying: the same backend/frontend **images** (built once in CI, pushed to GHCR) get promoted to prod - nothing about them changes. Only the Helm values (ConfigMap) and Vault secret paths ArgoCD wires up at the k8s layer differ. If you ever find yourself rebuilding an image differently for prod than for local, something in this doc has been violated - check here first.
