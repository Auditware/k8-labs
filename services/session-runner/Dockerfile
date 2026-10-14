# syntax=docker/dockerfile:1

# The session-runner image: a container whose job is to BE a throwaway k3s
# cluster (the target for namespace/RBAC-escape modules). Not a lab module -
# it holds no vulnerability and is reused by every escape lab. See
# services/session-runner/README.md and docs/architecture/nested-lab-rewrite.md.
#
# Static cgroup-namespace helper (see cgns-exec.c). Built static so it needs no
# libc inside the k3s image. Only exercised locally (runc); a no-op path in
# prod where Kata already isolates cgroups.
FROM debian:12-slim AS helper
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev \
    && rm -rf /var/lib/apt/lists/*
COPY cgns-exec.c /tmp/cgns-exec.c
RUN gcc -O2 -static -o /tmp/cgns-exec /tmp/cgns-exec.c

# Based on the upstream k3s image because it already bundles a working k3s +
# containerd + CNI - the same components k3d runs to give you a cluster in a
# container. We only swap in our own entrypoint (publish the kubeconfig for the
# backend). Pin to the k8s minor the prod control plane targets (v1.31).
FROM rancher/k3s:v1.31.5-k3s1

COPY --from=helper /tmp/cgns-exec /usr/local/bin/cgns-exec
# Baked foothold image (see foothold/Dockerfile), imported into every nested
# cluster on boot by the entrypoint so it never pulls at session time. This tar
# is a build artifact produced by `docker save` before this image is built (see
# the Tiltfile / build steps) and is gitignored - not checked in.
# All baked image tars: foothold.tar (the lab shell) + pause.tar
# (rancher/mirrored-pause, the pod sandbox image every nested pod needs) +
# coredns.tar (cluster DNS - k3s deploys CoreDNS but the airgapped cluster can't
# pull it, so without this every nested cluster has NO DNS) + tetragon.tar (the
# in-guest eBPF sensor for eBPF modules). Baking pause+coredns makes the nested
# cluster fully airgapped - it never pulls the sandbox or DNS image at session
# time, which otherwise fails/stalls behind the runner's egress NetworkPolicy.
# entrypoint.sh imports foothold+pause+coredns on every session but tetragon ONLY
# when SESSION_EBPF=1, so normal labs don't pay the heavier import - see
# entrypoint.sh and docs/architecture/ebpf-per-session.md.
COPY airgap/ /opt/airgap/
COPY entrypoint.sh /usr/local/bin/session-runner-entrypoint.sh
# rancher/k3s ships a busybox userland; make the script executable regardless of
# the host checkout's mode bits.
RUN chmod +x /usr/local/bin/session-runner-entrypoint.sh

# k3s serves its API here; the backend dials it (address rewritten to the pod
# IP by the entrypoint).
EXPOSE 6443

ENTRYPOINT ["/usr/local/bin/session-runner-entrypoint.sh"]
