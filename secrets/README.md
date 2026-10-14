# secrets/

Every service's environment variables are defined here, centrally - not spread
across `services/*/`. The `*.example` files are templates you copy; the real
`*.env` files are **gitignored and must never be committed**.

## Local development

```bash
cp secrets/backend.env.local.example  secrets/backend.env
cp secrets/frontend.env.example       secrets/frontend.env
# edit if you want real OAuth; the dummy defaults are enough to boot.
```

- `backend.env` is read by the backend and (in docker-compose) by Postgres.
- `frontend.env` is near-empty by design - the frontend talks to the backend
  over a relative `/api` path, so there's almost nothing environment-specific
  to set (see [docs/architecture/environments.md](../docs/architecture/environments.md)).

`backend.env.prod.example` is **reference only** - nothing reads it. It documents
the variable names/shapes a real deployment expects.

## Production / your own cluster

Nothing in this folder is meant to be committed with real values. For a real
deployment, split config and secrets:

- **Non-secret config** (ports, DB host/name, CORS origin) → a Helm ConfigMap
  (see `apps/backend/`).
- **Secrets** (DB password, JWT signing key, OAuth client secrets, registry pull
  credentials) → your secret manager of choice, injected at runtime. The
  reference deployment uses **HashiCorp Vault** (Vault Agent Injector), but SOPS,
  Sealed Secrets, or plain Kubernetes Secrets work the same way.

The infra-credential templates (`hcloud.env.example`, `ghcr.env.example`,
`grafana.env.example`, `argocd.env.example`) are only needed for the optional
pieces they name - provisioning (Terraform), private image pulls, the
observability stack, and ArgoCD with a private repo.

## Dockerfiles never bake secrets

No `Dockerfile` in this repo reads from this folder or copies an env file into an
image layer - anything written into a layer is extractable by anyone who can pull
the image. Env values are injected at **runtime only**:

- Locally: `docker-compose.yml`'s `env_file: ./secrets/*.env`.
- In a cluster: your secret manager + a Helm-managed ConfigMap supply them via the
  pod spec, not the image.

The one exception is the frontend's build-time `VITE_GOOGLE_CLIENT_ID` /
`VITE_GITHUB_CLIENT_ID` build args - Vite bakes `VITE_*` vars into the bundle at
build time, and OAuth client IDs aren't confidential (they're visible in every
redirect URL anyway). The actual client *secrets* never go near the frontend.

## Lab pods get no secrets

Lab pods run with `automountServiceAccountToken: false` and have no path to any
secret store - that's structural, not a convention. A lesson's planted/fake
credentials belong baked into that module's own image, never here (see
`services/labs/AUTHORING.md`).
