# apps/

What ArgoCD watches and syncs (`apps/**`). Each subfolder is one Helm chart/values pair or one `Application` manifest - independent, sync separately. See [docs/architecture/](../docs/architecture/) for how each component fits together, and the [self-hosting guide](../docs/self-hosting.md) for the deployment pipeline this feeds.

`labs/` is a catalog, not a single app - one training module per deployed image, sharing a common PodSecurity/NetworkPolicy baseline, with the specific image tag chosen at session-provision time by the Backend. `frontend/` is the one frontend SPA (catalog + auth + dashboard + the in-session Investigation UI view) - deliberately not split into separate frontend apps.

`infra/` is the separate, Terraform/CAPH-managed side (servers, cluster lifecycle) - nothing here touches that. `argocd/` (repo root, also separate) is ArgoCD's own bootstrap wiring - the `AppProject`/`Application` objects that point *at* `apps/**`, applied once via `kubectl`, not themselves synced by ArgoCD. A subfolder here being "one `Application` manifest" means an app ArgoCD deploys by pointing at someone else's chart (e.g. a third-party Helm chart you don't want to vendor) - not ArgoCD's own control-plane config.
