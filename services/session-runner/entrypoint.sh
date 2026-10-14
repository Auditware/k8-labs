#!/bin/sh
# session-runner entrypoint: boot a throwaway single-node k3s *inside this pod*,
# then publish its kubeconfig where the backend can fetch it.
#
# This container's whole job is to BE a disposable cluster - the attack target
# for namespace/RBAC-escape modules. It ships no vulnerability of its own; the
# lab content is applied into it later (see B3). Same image runs both places;
# the only environment difference is the pod's runtimeClassName (empty=runc
# local, kata-fc=microVM prod), set by the backend, not here.
set -eu

OUTPUT_KUBECONFIG="${OUTPUT_KUBECONFIG:-/output/kubeconfig}"
K3S_KUBECONFIG="/etc/rancher/k3s/k3s.yaml"

# The address the BACKEND will dial to reach this nested API. Loopback in the
# generated kubeconfig is useless outside the pod, so we rewrite it. The pod IP
# is injected via the downward API (SESSION_API_HOST); port stays 6443.
API_HOST="${SESSION_API_HOST:-127.0.0.1}"
API_PORT="${SESSION_API_PORT:-6443}"

# Re-exec ourselves inside a private cgroup namespace first (see cgns-exec.c).
# Needed in BOTH environments: under runc the nested kubelet would see the
# host's cgroups and containerd dies; under Kata the container's /sys/fs/cgroup
# is read-only and k3s can't "evacuate the root cgroup" - cgns-exec remounts a
# fresh writable cgroup2 either way. SESSION_RUNNER_CGNS guards the re-exec
# loop; SESSION_RUNNER_SKIP_CGNS=1 is an escape hatch, not normally set (proven
# necessary on Kata/AX41, A3).
if [ -z "${SESSION_RUNNER_CGNS:-}" ] && [ "${SESSION_RUNNER_SKIP_CGNS:-0}" != "1" ]; then
  SESSION_RUNNER_CGNS=1
  export SESSION_RUNNER_CGNS
  exec /usr/local/bin/cgns-exec "$0" "$@"
fi

# Guest fixups (harmless under runc, required under a Kata microVM - A3):
#  - /proc/sys is in runc's default readonlyPaths, but the nested kubelet's
#    ContainerManager must write kernel.panic / vm.overcommit_memory etc.
#    We hold CAP_SYS_ADMIN and are in our own mount ns (cgns-exec), so remount rw.
#  - Kata's minimal guest has no /dev/kmsg; the nested kubelet opens it. Create
#    it (fall back to a console symlink if mknod is denied).
mount -o remount,rw /proc/sys 2>/dev/null || true
[ -e /dev/kmsg ] || mknod /dev/kmsg c 1 11 2>/dev/null || ln -sf /dev/console /dev/kmsg 2>/dev/null || true

# eBPF sessions only: an in-guest Tetragon sensor reads /proc/kallsyms to resolve
# kprobe targets. The guest boots with kptr_restrict and perf_event_paranoid high,
# so kallsyms_show_value() hides every address (shows 0) and the sensor aborts
# ("no kernel symbols found ... kptr_restrict"). Reveal addresses to the sensor:
# kernel needs BOTH kptr_restrict=0 AND perf_event_paranoid<=1 for kallsyms to show
# real values to a container that lacks CAP_SYSLOG in the init userns. Guest-global
# and gone with the throwaway microVM, so nothing outside this session is affected.
# Gated on SESSION_EBPF so normal labs keep the tighter defaults. Found on the AX41
# Phase-4 gate - see docs/architecture/ebpf-per-session.md.
if [ "${SESSION_EBPF:-0}" = "1" ]; then
  echo 0 > /proc/sys/kernel/kptr_restrict 2>/dev/null || true
  echo 0 > /proc/sys/kernel/perf_event_paranoid 2>/dev/null || true
fi

# Preload baked lab images (see the Dockerfile's airgap stage) so nested pods
# find them locally instead of pulling per session (~1 min over the double-
# nested network). k3s imports any tars in agent/images/ on startup; the
# emptyDir mounted over /var/lib/rancher/k3s means we must drop them in now,
# after the mount, before k3s starts below.
#
# tetragon.tar is heavy and only eBPF modules use it, so importing it into EVERY
# nested cluster would slow every normal lab's boot for nothing. It's baked into
# the shared image (pulled to the node once) but imported only when the backend
# marks this an eBPF session (SESSION_EBPF=1, set per-module in buildRunnerPod).
# That's the "only eBPF pays" split - see docs/architecture/ebpf-per-session.md.
if ls /opt/airgap/*.tar >/dev/null 2>&1; then
  mkdir -p /var/lib/rancher/k3s/agent/images
  n=0
  for tar in /opt/airgap/*.tar; do
    case "$(basename "$tar")" in
      tetragon*.tar) [ "${SESSION_EBPF:-0}" = "1" ] || continue ;;
      # research.tar carries the ~200 MB kernel-exploitation toolchain + target
      # driver; only the two research labs use it, so import it only when the
      # backend marks this a research session (same split as tetragon above).
      research*.tar) [ "${SESSION_RESEARCH:-0}" = "1" ] || continue ;;
    esac
    cp "$tar" /var/lib/rancher/k3s/agent/images/ 2>/dev/null && n=$((n + 1)) || true
  done
  echo "session-runner: preloaded $n baked image tar(s) (ebpf=${SESSION_EBPF:-0} research=${SESSION_RESEARCH:-0})"
fi

# Drop the baked originals now that the tars this session actually uses are
# staged in agent/images. This is a CONFIDENTIALITY fix, not a disk one.
#
# /opt/airgap ships EVERY lab's images, because the runner image is shared by
# every session - so a normal lab's microVM was carrying research.tar: the whole
# kernel-driver research workstation (target driver source, starter fuzzer,
# guest kernel build tree) for a lab that is explicit-grant-only and unreleased.
# The gating above only decides what gets IMPORTED into the nested cluster; it
# never removed the tar from the guest. Meanwhile the escape modules
# (hostpath-escape, privileged-escape, node-token-harvest) exist precisely to
# break a nested pod out to the nested node - and the nested node IS this
# filesystem. So anyone who cleared the escape track could `tar xf` an unshipped
# lab straight out of a lab they were never granted. Isolation was never broken
# (it is all still inside the microVM, nowhere near the host), but the catalog
# gate leaked, which is how a customer reported the research labs before release.
#
# Unconditional and outside the block above so a failed glob can never skip it.
# Safe: k3s imports from agent/images, not from here, and reclaim_baked_tars
# drops those copies once containerd has them.
rm -rf /opt/airgap

# Wait for k3s to write its kubeconfig, then copy it out with the server
# address rewritten from 127.0.0.1 to the pod IP. Runs in the background while
# k3s itself stays PID 1 (below) so signals/reaping behave.
publish_kubeconfig() {
  i=0
  while [ ! -s "$K3S_KUBECONFIG" ]; do
    i=$((i + 1))
    if [ "$i" -gt 180 ]; then
      echo "session-runner: timed out waiting for $K3S_KUBECONFIG" >&2
      return 1
    fi
    sleep 1
  done
  mkdir -p "$(dirname "$OUTPUT_KUBECONFIG")"
  sed "s#https://127.0.0.1:6443#https://${API_HOST}:${API_PORT}#g" \
    "$K3S_KUBECONFIG" >"${OUTPUT_KUBECONFIG}.tmp"
  mv "${OUTPUT_KUBECONFIG}.tmp" "$OUTPUT_KUBECONFIG"
  echo "session-runner: published nested kubeconfig -> ${OUTPUT_KUBECONFIG} (server ${API_HOST}:${API_PORT})"
}

publish_kubeconfig &

# Reclaim the baked image tars once k3s has imported them. k3s imports every tar
# in agent/images/ into its containerd at startup and then LEAVES the tars there
# (it only re-reads them on a restart, which a throwaway session never does), so
# the guest holds BOTH the tar and the extracted image. On the heavy research
# image that doubles ~400 MB and can push the nested node past its ephemeral-
# storage eviction threshold (the workstation gets Evicted mid-session). Deleting
# the tars after import is safe: the images live in containerd's own store now, so
# imagePullPolicy: Never still finds them. Gate on the node being Ready, which
# only happens after the startup import completes. Bounded, best-effort.
reclaim_baked_tars() {
  IMGDIR=/var/lib/rancher/k3s/agent/images
  ls "$IMGDIR"/*.tar >/dev/null 2>&1 || return 0
  K3S_YAML=/etc/rancher/k3s/k3s.yaml
  # Wait for the node to be Ready. k3s imports agent/images/*.tar into containerd
  # while the agent comes up, which finishes before the kubelet reports the node
  # Ready (a healthy runtime is a precondition), so by Ready the images are in
  # containerd's store. Use the standalone /bin/kubectl + k3s.yaml - `k3s kubectl`
  # and `k3s ctr` are broken in this build (they see argv0 as "kubectl").
  i=0
  until kubectl --kubeconfig "$K3S_YAML" get nodes --no-headers 2>/dev/null \
        | awk '{print $2}' | grep -qx Ready; do
    i=$((i + 1))
    [ "$i" -gt 240 ] && { echo "session-runner: reclaim timed out; leaving tars" >&2; return 0; }
    sleep 2
  done
  # Belt-and-suspenders buffer so even the largest tar (research ~550 MB) has
  # finished importing before we drop the tars. Deleting is safe: the images live
  # in containerd's store now, so imagePullPolicy: Never still finds them.
  sleep 25
  n=$(ls "$IMGDIR"/*.tar 2>/dev/null | wc -l | tr -d ' ')
  rm -f "$IMGDIR"/*.tar
  echo "session-runner: reclaimed $n baked image tar(s) after import (freed nested-node disk)"
}
reclaim_baked_tars &

# Single-node, SQLite-backed, no add-ons a lab target has no use for. The API
# cert must be valid for whatever the backend dials, hence --tls-san.
#  - snapshotter=native: the nested containerd can't mount overlayfs on either
#    the outer overlay (runc/k3d) or Kata's virtio-fs/block rootfs; native
#    (plain copy) works everywhere. Override via SESSION_SNAPSHOTTER if needed.
#  - Distinct pod/service CIDRs from the OUTER cluster (k3d defaults 10.42/10.43)
#    or the nested kube-proxy hijacks the outer CoreDNS ClusterIP (10.43.0.10)
#    and breaks the runner's own DNS -> cascades into "can't pull pause".
exec /bin/k3s server \
  --disable=traefik,servicelb,metrics-server \
  --disable-helm-controller \
  --disable-cloud-controller \
  --write-kubeconfig-mode=0644 \
  --tls-san="${API_HOST}" \
  --snapshotter="${SESSION_SNAPSHOTTER:-native}" \
  --cluster-cidr="${SESSION_CLUSTER_CIDR:-10.52.0.0/16}" \
  --service-cidr="${SESSION_SERVICE_CIDR:-10.53.0.0/16}" \
  --cluster-dns="${SESSION_CLUSTER_DNS:-10.53.0.10}" \
  ${K3S_EXTRA_ARGS:-}
