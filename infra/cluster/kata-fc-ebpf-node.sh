#!/usr/bin/env bash
# Provision the kata-fc-ebpf runtime on a labs worker node (the AX41).
#
# Adds an eBPF-capable Kata/Firecracker runtime ALONGSIDE the existing kata-fc:
# a guest kernel with the flags an in-guest Tetragon CO-RE sensor needs, a
# configuration-fc-ebpf.toml pointing at it, and a kata-fc-ebpf containerd
# handler reusing the existing devmapper snapshotter. Everything else (default
# kata-fc) is untouched.
#
# WHY THIS EXISTS: the shipped -debug kernel is NOT sufficient - the Phase-4 gate
# on the AX41 (2026-07) found it lacks kprobe_multi (CONFIG_FPROBE) and
# acct_process (CONFIG_BSD_PROCESS_ACCT), and BTF is silently dropped at build
# time if pahole is missing. This script encodes the full working recipe so a
# node rebuild reproduces it instead of relying on the one hand-built kernel.
# See docs/architecture/ebpf-per-session.md ("Phase-4 gate PASSED - the full
# recipe").
#
# PREREQS: kata-fc already working on this node (kata-static installed, the
# -debug kernel present under /opt/kata/share/kata-containers, containerd's
# kata-fc handler + devmapper snapshotter configured - see kubeadm-2node.md).
# Run as root. Idempotent: safe to re-run; skips a step already done.
# The kernel compile takes ~20-30 min. The RuntimeClass itself is cluster-level,
# applied separately from the control plane:
#   kubectl apply -f infra/runtimeclass/kata-fc-ebpf.yaml
set -euo pipefail

# kata-static installs under /opt/kata/bin, and crictl/k8s bins under
# /usr/local/bin - neither is on a non-interactive shell's default PATH.
export PATH="/opt/kata/bin:/usr/local/bin:$PATH"

KATA_KDIR=/opt/kata/share/kata-containers
KATA_DEFAULTS=/opt/kata/share/defaults/kata-containers
CONTAINERD_CFG=/etc/containerd/config.toml
SRC_ROOT=/root/kata-ebpf-build

log() { echo ">>> $*"; }

# --- detect versions from what kata already installed ---
DEBUG_CFG=$(ls "$KATA_KDIR"/config-*-debug 2>/dev/null | head -1 || true)
[ -z "$DEBUG_CFG" ] && { echo "ERROR: no shipped -debug kernel config under $KATA_KDIR; is kata-fc set up?" >&2; exit 1; }
KVER=$(basename "$DEBUG_CFG" | sed -E 's/^config-(.*)-debug$/\1/')   # e.g. 6.18.35-197
KATA_VER=$(kata-runtime --version 2>/dev/null | sed -nE 's/.*kata-runtime\s*:\s*([0-9.]+).*/\1/p' | head -1 || true)
[ -z "$KATA_VER" ] && { echo "ERROR: could not detect kata-runtime version (kata-runtime on PATH? $(command -v kata-runtime || echo no))" >&2; exit 1; }
EBPF_KERNEL="$KATA_KDIR/vmlinux-${KVER}-ebpf"
log "kata $KATA_VER, guest kernel $KVER -> target $EBPF_KERNEL"

# --- 1. build + install the eBPF kernel (skip if already present with BTF) ---
if [ -f "$EBPF_KERNEL" ] && readelf -S "$EBPF_KERNEL" 2>/dev/null | grep -q '\.BTF'; then
  log "eBPF kernel already installed with BTF - skipping build"
else
  log "installing build deps (dwarves/pahole MUST be present before olddefconfig)"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq build-essential flex bison libelf-dev libssl-dev bc git curl zstd dwarves
  command -v yq >/dev/null || {
    log "installing yq (build-kernel.sh needs it)"
    curl -fsSL https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -o /usr/local/bin/yq
    chmod +x /usr/local/bin/yq
  }

  mkdir -p "$SRC_ROOT"; cd "$SRC_ROOT"
  [ -d kata-containers ] || {
    log "cloning kata-containers $KATA_VER"
    git clone --depth 1 --branch "$KATA_VER" https://github.com/kata-containers/kata-containers
  }
  cd kata-containers/tools/packaging/kernel

  log "build-kernel.sh setup (KERNEL_DEBUG_ENABLED=yes -> BTF + kprobes + tracefs base)"
  KERNEL_DEBUG_ENABLED=yes ./build-kernel.sh setup
  KSRC=$(ls -d kata-linux-* 2>/dev/null | head -1)
  [ -z "$KSRC" ] && { echo "ERROR: no kernel source dir after setup" >&2; exit 1; }
  cd "$KSRC"

  log "enable FPROBE (kprobe_multi) + BSD_PROCESS_ACCT (acct_process) + BTF"
  ./scripts/config --file .config -e FPROBE -e BSD_PROCESS_ACCT -e DEBUG_INFO_BTF
  make olddefconfig >/dev/null
  for f in CONFIG_FPROBE CONFIG_BSD_PROCESS_ACCT CONFIG_DEBUG_INFO_BTF; do
    grep -q "^${f}=y" .config || { echo "ERROR: $f not enabled after olddefconfig (pahole missing?)" >&2; exit 1; }
  done

  log "compiling vmlinux ($(nproc) cores, ~20-30 min)"
  make -j"$(nproc)" vmlinux
  readelf -S vmlinux | grep -q '\.BTF' || { echo "ERROR: built vmlinux has no .BTF section" >&2; exit 1; }
  install -m 0644 vmlinux "$EBPF_KERNEL"
  log "installed $EBPF_KERNEL"
fi

# --- 2. configuration-fc-ebpf.toml = configuration-fc.toml + the eBPF kernel ---
FC_EBPF_TOML="$KATA_DEFAULTS/configuration-fc-ebpf.toml"
log "writing $FC_EBPF_TOML"
cp "$KATA_DEFAULTS/configuration-fc.toml" "$FC_EBPF_TOML"
sed -i "s#^kernel = .*#kernel = \"$EBPF_KERNEL\"#" "$FC_EBPF_TOML"
grep -q "^kernel = \"$EBPF_KERNEL\"$" "$FC_EBPF_TOML" || { echo "ERROR: kernel= not set in $FC_EBPF_TOML" >&2; exit 1; }

# --- 3. containerd kata-fc-ebpf handler (idempotent append, reuses devmapper) ---
if grep -q "runtimes.kata-fc-ebpf\]" "$CONTAINERD_CFG"; then
  log "containerd kata-fc-ebpf handler already present"
else
  log "appending kata-fc-ebpf handler to $CONTAINERD_CFG + restarting containerd"
  cp "$CONTAINERD_CFG" "$CONTAINERD_CFG.bak.pre-ebpf.$(date +%s)"
  cat >> "$CONTAINERD_CFG" <<TOML

        [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.kata-fc-ebpf]
  runtime_type = "io.containerd.kata.v2"
  snapshotter = "devmapper"
          [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.kata-fc-ebpf.options]
    ConfigPath = "$FC_EBPF_TOML"
TOML
  systemctl restart containerd
  sleep 4
  if systemctl is-active --quiet containerd && crictl info >/dev/null 2>&1; then
    crictl info 2>/dev/null | grep -q "kata-fc-ebpf" && log "kata-fc-ebpf runtime registered" || echo "WARN: handler not visible in crictl info" >&2
  else
    LATEST=$(ls -t "$CONTAINERD_CFG".bak.pre-ebpf.* | head -1)
    echo "ERROR: containerd unhealthy after restart - restoring $LATEST" >&2
    cp "$LATEST" "$CONTAINERD_CFG"; systemctl restart containerd
    exit 1
  fi
fi

log "DONE. Apply the RuntimeClass from the control plane if not already:"
log "  kubectl apply -f infra/runtimeclass/kata-fc-ebpf.yaml"
