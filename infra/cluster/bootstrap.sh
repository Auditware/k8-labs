#!/usr/bin/env bash
# One-command platform bootstrap for a freshly-provisioned prod cluster.
#
# Split of responsibility:
#   - This script installs the *substrate* - the upstream components (CNI,
#     ArgoCD, cert-manager controller, Vault, Tetragon, observability) that
#     GitOps can't manage: the CNI must exist before any pod schedules,
#     ArgoCD can't be the thing that first creates ArgoCD, and the rest
#     install cluster-wide CRDs/webhooks the tight k8labs AppProject rightly
#     forbids. These use the exact versions/values validated locally in
#     docs/architecture/environments.md and rarely change.
#   - ArgoCD's App-of-Apps (argocd/root.yaml) then owns everything that DOES
#     change on every push - our charts (backend, frontend, postgres, labs,
#     cert-manager issuers, tetragon policy) - and keeps them converged.
#
# Idempotent: every step is `helm upgrade --install` / `kubectl apply`, safe
# to re-run. Prereqs (infra/cluster/README.md): the CAPH cluster exists,
# KUBECONFIG points at it, the labs node is labelled + has the kata-fc
# RuntimeClass. This is the platform half; it does not provision infra.
set -euo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ARCH="$REPO_ROOT/docs/architecture"

echo "==> [1/8] Cilium (CNI) - nodes stay NotReady until this lands"
helm repo add cilium https://helm.cilium.io/ >/dev/null && helm repo update cilium >/dev/null
# No kubeProxyReplacement=false here: that was a k3d-only workaround (k3s ships
# its own kube-proxy). A real kubeadm cluster uses Cilium's default.
helm upgrade --install cilium cilium/cilium --version 1.16.5 --namespace kube-system \
  --set ipam.mode=kubernetes --set operator.replicas=1
cilium status --wait || true

echo "==> [2/8] cert-manager controller"
helm repo add jetstack https://charts.jetstack.io >/dev/null && helm repo update jetstack >/dev/null
helm upgrade --install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace \
  --version v1.16.2 --set crds.enabled=true

echo "==> [3/8] Vault (Raft HA) - still needs manual init/unseal, see docs/self-hosting.md"
helm repo add hashicorp https://helm.releases.hashicorp.com >/dev/null && helm repo update hashicorp >/dev/null
helm upgrade --install vault hashicorp/vault --namespace vault --create-namespace \
  --set server.dataStorage.enabled=true --set server.dataStorage.size=1Gi \
  --set server.ha.enabled=true --set server.ha.replicas=1 \
  --set server.ha.raft.enabled=true --set server.ha.raft.setNodeId=true \
  --set injector.enabled=true

echo "==> [4/8] Tetragon (eBPF EDR)"
helm upgrade --install tetragon cilium/tetragon --namespace kube-system --version 1.7.0

echo "==> [5/8] Observability (Loki + Grafana + Alloy)"
helm repo add grafana https://grafana.github.io/helm-charts >/dev/null && helm repo update grafana >/dev/null
helm upgrade --install loki grafana/loki --namespace observability --create-namespace --version 7.0.0 \
  --set deploymentMode=SingleBinary --set loki.auth_enabled=false \
  --set loki.commonConfig.replication_factor=1 --set loki.storage.type=filesystem \
  --set loki.useTestSchema=true --set singleBinary.replicas=1 \
  --set read.replicas=0 --set write.replicas=0 --set backend.replicas=0 \
  --set gateway.enabled=false --set test.enabled=false --set lokiCanary.enabled=false \
  --set chunksCache.enabled=false --set resultsCache.enabled=false
# Grafana admin creds from secrets/grafana.env (git secret reveal + source it
# first), so prod never ships admin/admin and a redeploy keeps the real
# password instead of resetting it. grafana-values.yaml keeps admin/admin only
# as the LOCAL default; the --set below makes prod read this Secret instead.
# Fail loudly if unset rather than silently shipping admin/admin. Never echoed.
kubectl create namespace observability --dry-run=client -o yaml | kubectl apply -f - >/dev/null
: "${GRAFANA_ADMIN_PASSWORD:?set it in secrets/grafana.env (git secret reveal) before bootstrapping Grafana}"
kubectl -n observability create secret generic grafana-admin \
  --from-literal=admin-user="${GRAFANA_ADMIN_USER:-admin}" \
  --from-literal=admin-password="$GRAFANA_ADMIN_PASSWORD" \
  --dry-run=client -o yaml | kubectl apply -f -
helm upgrade --install grafana grafana/grafana --namespace observability -f "$ARCH/grafana-values.yaml" \
  --set admin.existingSecret=grafana-admin --set admin.userKey=admin-user --set admin.passwordKey=admin-password
helm upgrade --install alloy grafana/alloy --namespace observability --version 1.10.1 \
  --set-file alloy.configMap.content="$ARCH/alloy-config.river"

echo "==> [6/8] ArgoCD (the GitOps engine itself)"
helm repo add argo https://argoproj.github.io/argo-helm >/dev/null && helm repo update argo >/dev/null
helm upgrade --install argocd argo/argo-cd --version 7.7.11 --namespace argocd --create-namespace \
  --set 'configs.params.server\.insecure=true' --set 'configs.params.server\.rootpath=/admin/argocd'
kubectl -n argocd rollout status deploy/argocd-server --timeout=180s

echo "==> [7/8] Repo credentials + AppProject"
# Private-repo read credential ArgoCD needs. Expects GITHUB_PAT in the env
# (source secrets/argocd.env first), never echoed.
if [[ -n "${GITHUB_PAT:-}" ]]; then
  kubectl create secret generic k8labs-repo-creds -n argocd \
    --from-literal=type=git --from-literal=url=https://github.com/Auditware/k8-labs.git \
    --from-literal=username=x-access-token --from-literal=password="$GITHUB_PAT" \
    --dry-run=client -o yaml \
    | kubectl label --local -f - argocd.argoproj.io/secret-type=repository -o yaml | kubectl apply -f -
else
  echo "    (GITHUB_PAT unset - skipping repo creds; set it if the repo is private)"
fi
kubectl apply -f "$REPO_ROOT/argocd/project.yaml"

echo "==> [8/8] Root App-of-Apps - ArgoCD now owns the app tier from git"
kubectl apply -f "$REPO_ROOT/argocd/root.yaml"

cat <<'EOF'

Substrate up; ArgoCD is converging the app tier from git.
Watch:  kubectl -n argocd get applications -w

Still manual (they need real secret VALUES this script must never hold - see
docs/self-hosting.md):
  - Vault init/unseal + write secret/backend
  - GHCR image pull secret (private images)
  - cert-manager issuer -> ACME prod (your email + live DNS)
  - DNS A record k8labs.example.com -> ingress IP (gates the TLS cert)
EOF
