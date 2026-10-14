# Adding a new training module

A module is **not a container image**. It's a bundle of Kubernetes manifests
applied *into* the disposable nested k3s cluster each session boots (see
[nested-lab-rewrite.md](../../../docs/architecture/nested-lab-rewrite.md)).
Winning a lab compromises only that throwaway cluster.

Fill in:

1. **The manifest bundle** - a multi-document YAML file at
   `services/backend/internal/labs/modules/<slug>.yaml`, `//go:embed`-ed in
   `internal/labs/modules.go`. It creates everything the student attacks:
   namespaces, the vulnerable Deployments/ServiceAccounts/Roles/RoleBindings,
   the planted misconfiguration, and the flag Secret. The backend applies it via
   the nested kubeconfig - **not** ArgoCD, which never sees the nested cluster.

2. **The entry pod** - the student's starting foothold, created by that same
   bundle. Name it in the registry's `NestedSpec`
   (`EntryNamespace`/`EntryPod`). The terminal execs into it over a websocket;
   there is no ttyd anywhere in the session path any more.

3. **Register it** in `services/backend/internal/labs/registry.go` - slug,
   display name, description, hint, and the `NestedSpec`. (This moves to a
   Postgres catalog with the content model - see
   [lab-session-rewrite.md](../../../docs/architecture/lab-session-rewrite.md).)
   Don't duplicate per-module config under `apps/labs/`, which holds only the
   static runner-namespace/RBAC/NetworkPolicy scaffolding.

4. **`README.md`** - document the vulnerability class (CWE reference if
   applicable), the exploit path, and what a defender would look for. Note that
   under Kata in prod, host-level Tetragon **cannot** see inside the nested
   cluster (see `apps/tetragon/templates/process-creds-changed.yaml`), so "what
   the EDR observes" is a design question here, not a given.

5. **The course** - the guided left-pane lesson. See
   [lab-session-rewrite.md](../../../docs/architecture/lab-session-rewrite.md)
   for the step schema and diagram DSL.

## PodSecurity

Lab content runs **inside** the nested cluster, governed by that cluster's own
admission - not the outer cluster's. The outer `labs-runners` namespace is
`privileged` because the runner boots k3s, and it holds only runner pods, never
lab content. So a module whose lesson needs `allowPrivilegeEscalation` sets that
in its own manifests; there is no outer restricted/baseline split to opt into any
more (that went away with the legacy pod shape - see lab-session-rewrite.md, F1).

## Secrets: there aren't any, and that's on purpose

The nested cluster has no route to Vault or any real platform secret, and
`apps/labs/templates/networkpolicy-runners.yaml` blocks the whole nested cluster
from reaching the outer API, Postgres, the backend, and every other session's
runner. This is structural, not a convention someone could accidentally skip.
Don't add a `secrets/labs.env` or similar; there's nothing that belongs there.

If a module's lesson involves a planted credential ("find the leaked API key"),
that value is **bait, not a secret** - put it directly in the module's own
manifest bundle. It has no business anywhere under `secrets/`, which exists to
protect values that are actually sensitive.

Worked example: `ns-escape` -
`services/backend/internal/labs/modules/ns-escape.yaml`.
