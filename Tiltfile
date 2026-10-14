# Tiltfile - hot-reload local dev against the real k3d cluster.
#
# `tilt up` from the repo root (cluster + Cilium CNI must already exist -
# see docs/architecture/environments.md's "Tilt against k3d" section for
# the full k3d-create + Cilium-install commands).
#
# What this manages: postgres, the session-runner namespace+RBAC, and the
# backend (with live_update - saving a .go file syncs the rebuilt binary into
# the running pod and restarts the process, no docker build/push/helm-upgrade
# round trip). The session-runner image builds normally (it's the nested-k3s
# arena, not part of the active edit loop).
#
# Frontend is deliberately NOT deployed here - same as docker-compose, run
# it natively with `bun run dev` (services/frontend) for Vite's own HMR;
# its /api proxy talks to the backend through Traefik on :8080 either way.

load('ext://restart_process', 'docker_build_with_restart')

# k3d's registry is the same container reachable at two different names:
# localhost:5000 from the host (where `docker build`/push run), k8labs-
# registry:5000 from inside the cluster (where pods pull from). Tilt needs
# both - see k3d's own Tilt integration docs for this exact pattern.
default_registry('localhost:5000', host_from_cluster='k8labs-registry:5000')

# Guard the two local prerequisites the backend image pipeline needs, so a
# save-to-pod loop never stalls on a stale pod: (1) the `alpine:3` build base is
# present locally (else `docker build` hangs on Docker Hub when the daemon can't
# reach it), and (2) the `k8labs-registry` -> IP alias is in every node's
# /etc/hosts (k3d only injects it on `k3d cluster start`, so a raw docker restart
# drops it and new-tag pulls fail with "no such host"). Both idempotent, run at
# Tiltfile eval before any build below; neither fails the build. Script header
# has the full story.
local('bash scripts/ensure-registry-dns.sh')

def read_env_file(path):
    result = {}
    if not os.path.exists(path):
        return result
    for line in str(read_file(path)).splitlines():
        line = line.strip()
        if not line or line.startswith('#') or '=' not in line:
            continue
        key, value = line.split('=', 1)
        result[key.strip()] = value.strip()
    return result

# Same file docker-compose/bun-dev already read from - see secrets/README.md.
# Real values (Google/GitHub OAuth, admin seed) flow into the cluster the
# same way they flowed into the docker-compose backend, just via Helm --set
# instead of env_file.
secrets = read_env_file('secrets/backend.env')

# --- cert-manager's ClusterIssuer chain (cert-manager itself is a cluster
# prerequisite, installed the same way as Cilium - see
# docs/architecture/environments.md). Self-signed root -> CA ClusterIssuer
# locally; real Let's Encrypt once the public domain exists. ---
k8s_yaml(helm(
    'apps/cert-manager',
    name='cert-manager-issuers',
))

# --- Postgres ---
k8s_yaml(helm(
    'apps/postgres',
    name='postgres',
))
k8s_resource('postgres-postgres', labels=['data'])

# LOCAL DEV ONLY: keep Vault unsealed. Raft storage re-seals on every Vault pod
# restart (the memory-tight VM evicts it), and a fresh backend pod then hangs at
# its vault-agent init until unsealed. This watcher unseals from the local keys
# file within seconds, so a re-seal stops being a manual interruption. Prod keeps
# unseal manual BY DESIGN - never wire this there. See scripts/vault-unseal-local.sh.
# Vault itself is a cluster prerequisite,
# not managed here, so this only watches - it never installs or owns it.
local_resource(
    'vault-unseal',
    serve_cmd='bash scripts/vault-unseal-local.sh --watch',
    labels=['data'],
)

# --- Labs namespace + RBAC (static - the per-session pods themselves are
# created dynamically by the backend via client-go, not by Tilt/Helm) ---
k8s_yaml(helm(
    'apps/labs',
    name='labs',
    set=[
        'backend.serviceAccountName=backend-backend',
        'backend.namespace=default',
    ],
))

# --- Tetragon's detection policy for the session-runner namespace (Tetragon
# itself is a cluster prerequisite, installed the same way as Cilium - see
# docs/architecture/environments.md). Needs the labs-runners namespace from the
# chart above to already exist. Locally (runc) this sees into the nested
# cluster; under Kata in prod it does not - see the policy's own comment. ---
k8s_yaml(helm(
    'apps/tetragon',
    name='tetragon-policies',
))

# --- session-runner - the nested-k3s image (arena for escape modules). Same
# local_resource rationale as the lab images above: never appears in applied
# YAML (the Backend builds the runner pod via client-go, B1), so a plain
# build+push instead of docker_build. See services/session-runner/README.md.
local_resource(
    'session-runner-image',
    # Two steps: build the tiny foothold image and `docker save` it into the
    # runner's build context (airgap/, gitignored), so the runner bakes it in
    # and every nested cluster imports it on boot - no per-session image pull
    # (see services/session-runner/foothold + entrypoint.sh's preload).
    'docker build -t k8labs-foothold:1.31 services/session-runner/foothold && ' +
    'mkdir -p services/session-runner/airgap && ' +
    'docker save k8labs-foothold:1.31 -o services/session-runner/airgap/foothold.tar && ' +
    # research: the kernel-driver research workstation (foothold + a real
    # exploitation toolchain + the target driver/fuzzer sources). FROM foothold,
    # so it must build after it. Baked like tetragon but imported only when the
    # backend sets SESSION_RESEARCH=1 (entrypoint.sh) - heavy (~200 MB toolchain)
    # and only the two research labs use it. See docs/architecture/kasan-research-lab.md.
    # TREE_MODE=none: skip the heavy x86 kernel build tree locally (prod-only, and
    # unbuildable on an arm64 dev Mac). CI builds the full tree (default).
    'docker build --build-arg TREE_MODE=none -t k8labs-research:1.0 services/session-runner/research && ' +
    'docker save k8labs-research:1.0 -o services/session-runner/airgap/research.tar && ' +
    # pause: baked so the nested cluster never pulls its sandbox image at
    # session time (matches CI; see services/session-runner/Dockerfile).
    'docker pull rancher/mirrored-pause:3.6 && ' +
    'docker save rancher/mirrored-pause:3.6 -o services/session-runner/airgap/pause.tar && ' +
    # tetragon: the in-guest eBPF sensor for eBPF modules (Module.EBPF). Baked
    # into the shared image but imported into a nested cluster only when the
    # backend sets SESSION_EBPF=1 (entrypoint.sh), so normal labs keep fast boot.
    # Heavy (~hundreds of MB) - see docs/architecture/ebpf-per-session.md.
    'docker pull quay.io/cilium/tetragon:v1.7.0 && ' +
    'docker save quay.io/cilium/tetragon:v1.7.0 -o services/session-runner/airgap/tetragon.tar && ' +
    # coredns: k3s deploys CoreDNS as cluster DNS, but the airgapped nested cluster
    # can't pull it - without this it sits in ErrImagePull and every nested cluster
    # runs with NO DNS, breaking any lab that resolves a Service by name (the recon
    # track) or hits kubernetes.default.svc. Imported for every session like
    # foothold/pause (not gated). Ref must track the k3s version in
    # services/session-runner/Dockerfile (v1.31.5-k3s1 -> coredns 1.12.0).
    'docker pull rancher/mirrored-coredns-coredns:1.12.0 && ' +
    'docker save rancher/mirrored-coredns-coredns:1.12.0 -o services/session-runner/airgap/coredns.tar && ' +
    'docker build -t localhost:5000/session-runner:dev services/session-runner && ' +
    'docker push localhost:5000/session-runner:dev',
    deps=[
        'services/session-runner/Dockerfile',
        'services/session-runner/entrypoint.sh',
        'services/session-runner/foothold/Dockerfile',
        'services/session-runner/research',
    ],
    labels=['labs'],
)

# --- Backend - the actual hot-reload loop ---
# Blocking compile at Tiltfile-eval time - guarantees bin/backend exists
# before the image build below ever runs. Found the hard way that
# k8s_resource's resource_deps only gates the *deploy* step, not the
# image build itself, so relying on local_resource alone raced the very
# first `tilt up`/`tilt ci` and failed with "bin/backend: not found".
#
# Stamp the short git SHA into the binary (buildinfo.SHA) so /healthz and the
# startup log report which commit is running - a dead Tilt serving a stale binary
# then shows as a SHA that lags HEAD, instead of silently freezing local behind.
backend_build = 'CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags "-X backend/internal/buildinfo.SHA=$(git rev-parse --short HEAD 2>/dev/null || echo dev)" -o bin/backend ./cmd/server'

local(backend_build, dir='services/backend')

local_resource(
    'backend-compile',
    backend_build,
    dir='services/backend',
    deps=['services/backend'],
    ignore=['services/backend/bin'],
    labels=['backend'],
)

docker_build_with_restart(
    'backend',
    context='services/backend',
    dockerfile='services/backend/Dockerfile.dev',
    entrypoint=['/app/backend'],
    only=['./bin/backend'],
    live_update=[
        sync('services/backend/bin/backend', '/app/backend'),
    ],
)

k8s_yaml(helm(
    'apps/backend',
    name='backend',
    set=[
        # Real Vault, not the localSecret fallback - see
        # docs/architecture/environments.md for the install/init/unseal
        # steps and how secret/data/backend gets populated. Google/GitHub
        # OAuth is now the only login path locally too (the seeded
        # admin/admin account is gone), so local Vault needs real
        # google_client_id/secret values - see environments.md.
        'vault.enabled=true',
        'image.repository=backend',
        'image.tag=dev',
        'config.databaseHost=postgres-postgres.default.svc.cluster.local',
        # LOCAL DEV ONLY: mount the username/password login route (localhost
        # testing without OAuth). Prod's chart default is "" - never mounted.
        # See services/backend internal/api/devlogin.go.
        'config.devLogin=1',
        'ingress.host=localhost',
        # Exercises the real cert-manager issuance path locally against the
        # self-signed CA chain apps/cert-manager sets up above - see
        # docs/architecture/environments.md for how to fetch/trust that
        # root CA to actually browse https://localhost:8443 without -k.
        'ingress.tls.enabled=true',
        # Nested-shape modules: the session-runner image (built by the
        # session-runner-image local_resource above) via the in-cluster
        # registry name, and its privileged namespace. labsRuntimeClass stays
        # empty below so the runner takes the runc (privileged) path locally.
        'config.labsRunnerImage=k8labs-registry:5000/session-runner:dev',
        'config.labsRunnerNamespace=labs-runners',
        # k3d has no Kata - override the chart's prod "kata-fc" default back
        # to empty so lab pods run as plain runc locally (see values.yaml's
        # labsRuntimeClass comment). Without this, local session pods would
        # reference a RuntimeClass that doesn't exist and never schedule.
        'config.labsRuntimeClass=',
        # Same for the eBPF class (chart default kata-fc-ebpf): k3d has no Kata,
        # and locally runc's host kernel already has BTF, so an eBPF module runs
        # runc-privileged with no custom kernel. Empty here -> runtimeClassFor
        # falls back to the (also-empty) labsRuntimeClass = the runc path.
        'config.labsRuntimeClassEbpf=',
        # Same for the research classes (chart defaults kata-fc-research-{dev,hard}):
        # no Kata on k3d, and the KASAN kernel is prod-only, so a research module
        # runs runc-privileged locally with the host kernel - the plumbing is
        # exercised, the custom kernel is not. Empty -> runtimeClassFor falls back
        # to the runc path. See docs/architecture/kasan-research-lab.md.
        'config.labsRuntimeClassResearchDev=',
        'config.labsRuntimeClassResearchHard=',
        # Matches the label applied to the k3d agent node - see
        # docs/architecture/environments.md's "labs node" step.
        'config.labsNodeSelector=k8labs.io/role=labs',
        'config.corsOrigin=' + secrets.get('CORS_ORIGIN', 'http://localhost:7373'),
        # Dockerfile.dev's alpine base uses UID 100 (matches frontend's, see
        # its own Dockerfile), and live_update's file sync needs a writable
        # root fs - found this out by watching the sync fail with "tar:
        # can't remove old file: Read-only file system" against the chart's
        # normal hardened default. Prod/real installs never pass these, so
        # they keep the distroless-appropriate defaults in values.yaml.
        'securityContext.runAsUser=100',
        'securityContext.readOnlyRootFilesystem=false',
    ],
))
k8s_resource('backend-backend', resource_deps=['postgres-postgres', 'backend-compile'], labels=['backend'])

# Drift check: confirm the running backend's SHA matches HEAD. Runs once after the
# backend is up (and re-runnable from the Tilt UI). If Tilt ever dies mid-session
# the pod keeps serving a stale binary silently - so also run `scripts/local-status.sh`
# by hand any time local looks behind; it catches a dead Tilt that this resource
# can't (a dead Tilt runs nothing). See buildinfo / the /healthz sha field.
local_resource(
    'local-status',
    'bash scripts/local-status.sh',
    resource_deps=['backend-backend'],
    labels=['backend'],
)
