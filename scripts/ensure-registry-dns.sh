#!/usr/bin/env bash
# Ensure the two local prerequisites Tilt's backend image pipeline needs, so a
# save-to-pod loop never silently stalls on a stale pod:
#   1. the alpine:3 build base is present locally (see the dev-base block below), and
#   2. the in-cluster registry name resolves from every k3d node (below).
# Both run idempotently on every `tilt up` (the Tiltfile's top-level local()),
# and neither ever fails the build - worst case they print a WARN and exit 0.
#
# The registry-name failure this guards against: k3d writes a `k8labs-registry` -> IP alias into
# each node's /etc/hosts when it *starts* the cluster, but a raw `docker start`
# (Docker Desktop restart, reboot, a bare `docker start` of the containers)
# brings the nodes back up WITHOUT re-injecting it. The registry stays alive and
# reachable by IP the whole time - only the name lookup is gone. It hides until a
# NEW image tag has to be pulled (cached tags keep running), then surfaces as:
#   ErrImagePull ... dial tcp: lookup k8labs-registry: no such host
# and Tilt's rebuild silently "reverts" to the old image. (The backend bakes its
# course HTML into the binary, so a stale backend also = stale lesson HTML.)
#
# This re-adds the alias idempotently on every `tilt up` (see the Tiltfile's
# top-level local() call). It never fails the build - if the cluster/registry
# isn't up yet, it just says so and exits 0.
set -uo pipefail

CLUSTER=k8labs-local
REGISTRY=k8labs-registry

if ! command -v docker >/dev/null 2>&1; then
  echo "registry-dns: docker not found, skipping"; exit 0
fi

# --- dev-base: make the local dev image base present, so Tilt's image build
# never hangs on Docker Hub. services/backend/Dockerfile.dev is `FROM alpine:3`;
# when the Docker daemon can't reach registry-1.docker.io (a Docker Desktop VM
# glitch - the host resolves Hub fine, the daemon times out), `docker build`
# blocks on "load metadata for alpine:3" and every backend rebuild fails, leaving
# the running pod on a stale binary with no error. The dev image only ever runs
# on a local cluster, so any recent alpine works as its base. Runs before the
# registry checks below because it needs only docker, not a running cluster. ---
BASE=alpine:3
if docker image inspect "$BASE" >/dev/null 2>&1; then
  echo "dev-base: $BASE present"
else
  # Prefer tagging an alpine already on disk (no network). Only if none exists do
  # we try a short, bounded pull - so a dead Hub can never hang the Tiltfile eval.
  SRC=$(docker images alpine --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep -v '<none>' | head -1)
  if [ -n "${SRC:-}" ]; then
    docker tag "$SRC" "$BASE" >/dev/null 2>&1 \
      && echo "dev-base: tagged $SRC -> $BASE (offline, no Hub hit)" \
      || echo "dev-base: WARN could not tag $SRC -> $BASE"
  elif timeout 20 docker pull -q "$BASE" >/dev/null 2>&1; then
    echo "dev-base: pulled $BASE"
  else
    echo "dev-base: WARN no local alpine and Hub unreachable - run 'docker tag <any-alpine> $BASE' or restart Docker Desktop"
  fi
fi

# Registry container present? If not, the cluster likely isn't up - nothing to do.
if ! docker inspect "$REGISTRY" >/dev/null 2>&1; then
  echo "registry-dns: '$REGISTRY' container not found, skipping (cluster not up?)"; exit 0
fi

# The k3d docker network for this cluster (e.g. k3d-k8labs-local). Derive it from
# the registry's own networks so a renamed cluster still works.
NET=$(docker inspect -f '{{range $k,$v := .NetworkSettings.Networks}}{{println $k}}{{end}}' "$REGISTRY" 2>/dev/null | grep '^k3d-' | head -1)
if [ -z "${NET:-}" ]; then
  echo "registry-dns: '$REGISTRY' not on a k3d-* network yet, skipping"; exit 0
fi

# Registry's IP on that network - the address the nodes must resolve to.
IP=$(docker inspect -f "{{ (index .NetworkSettings.Networks \"$NET\").IPAddress }}" "$REGISTRY" 2>/dev/null)
if [ -z "${IP:-}" ]; then
  echo "registry-dns: could not read '$REGISTRY' IP on $NET, skipping"; exit 0
fi

# Every server + agent node in this cluster (the loadbalancer/registry don't pull).
NODES=$(docker ps -q \
  --filter "label=k3d.cluster=$CLUSTER" \
  --filter "label=k3d.role=server" \
  --filter "label=k3d.role=agent" 2>/dev/null)
# --filter with two k3d.role values ORs them in some docker versions and ANDs in
# others; fall back to listing the cluster's nodes and dropping lb/registry.
if [ -z "${NODES:-}" ]; then
  NODES=$(docker ps --format '{{.Names}}' --filter "label=k3d.cluster=$CLUSTER" 2>/dev/null \
    | grep -E -- '-(server|agent)-' )
fi
if [ -z "${NODES:-}" ]; then
  echo "registry-dns: no server/agent nodes found for cluster '$CLUSTER', skipping"; exit 0
fi

for node in $NODES; do
  # Idempotent, and self-correcting if the registry IP changed: only rewrite when
  # the exact line is absent. `cat tmp > /etc/hosts` truncates-in-place (the file
  # is a docker-managed bind mount, so mv/sed -i would fail with "resource busy").
  docker exec "$node" sh -c '
    ip="'"$IP"'"; name="'"$REGISTRY"'"
    if ! grep -q "^${ip} ${name}\$" /etc/hosts; then
      grep -v " ${name}\$" /etc/hosts > /etc/hosts.tmp 2>/dev/null || true
      cat /etc/hosts.tmp > /etc/hosts && rm -f /etc/hosts.tmp
      echo "${ip} ${name}" >> /etc/hosts
    fi
  ' 2>/dev/null \
    && echo "registry-dns: $(docker inspect -f '{{.Name}}' "$node" | sed 's#^/##') -> $IP $REGISTRY" \
    || echo "registry-dns: WARN could not patch node $node"
done

exit 0
