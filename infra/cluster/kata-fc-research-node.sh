#!/usr/bin/env bash
# Provision the kata-fc-research-{dev,hard} runtimes on a labs worker node (the AX41).
#
# Adds TWO KASAN-instrumented Kata/Firecracker runtimes ALONGSIDE the existing
# kata-fc / kata-fc-ebpf: a guest kernel whose own memory-safety instrumentation
# is the point of the lab (the guest KERNEL is the exploitation target). Both
# share one instrumented base (KASAN + KCOV + BTF + kprobes + loadable modules);
# they differ only in a mitigation fragment:
#   - research-dev : mitigations OFF   (KASLR/freelist-hardening/cache-random off) - land the technique
#   - research-hard: mitigations ON    (KASLR, hardened freelist, randomized kmalloc caches,
#                    STATIC_USERMODEHELPER) - prove it survives real hardening
# Everything else (default kata-fc, kata-fc-ebpf) is untouched.
#
# This is the research analog of kata-fc-ebpf-node.sh and reuses the SAME
# machinery: build-kernel.sh, the devmapper snapshotter, a per-runtime
# configuration TOML, and an idempotent containerd handler append. See
# docs/architecture/kasan-research-lab.md and docs/architecture/ebpf-per-session.md.
#
# WHY BUILD ON THE BOX: the research kernel is x86_64 (the AX41's arch) and is
# prod-only - the arm64 dev Mac cannot build or run it (runc shares the host
# kernel; no custom kernel loads there). The kernel behaviour (KASAN firing on a
# UAF) is exercised only here.
#
# PREREQS: kata-fc already working on this node (kata-static installed, the
# -debug kernel present under /opt/kata/share/kata-containers, containerd's
# kata-fc handler + devmapper snapshotter configured - see kubeadm-2node.md).
# Run as root. Idempotent: safe to re-run; skips a kernel/handler already done.
# Each kernel compile takes ~20-30 min (two here, ~40-60 min total). The
# RuntimeClasses are cluster-level, applied separately from the control plane:
#   kubectl apply -f infra/runtimeclass/kata-fc-research-dev.yaml
#   kubectl apply -f infra/runtimeclass/kata-fc-research-hard.yaml
set -euo pipefail

# kata-static installs under /opt/kata/bin, and crictl/k8s bins under
# /usr/local/bin - neither is on a non-interactive shell's default PATH.
export PATH="/opt/kata/bin:/usr/local/bin:$PATH"

KATA_KDIR=/opt/kata/share/kata-containers
KATA_DEFAULTS=/opt/kata/share/defaults/kata-containers
CONTAINERD_CFG=/etc/containerd/config.toml
SRC_ROOT=/root/kata-research-build

log() { echo ">>> $*"; }

# --- detect versions from what kata already installed (same as the ebpf script) ---
DEBUG_CFG=$(ls "$KATA_KDIR"/config-*-debug 2>/dev/null | head -1 || true)
[ -z "$DEBUG_CFG" ] && { echo "ERROR: no shipped -debug kernel config under $KATA_KDIR; is kata-fc set up?" >&2; exit 1; }
KVER=$(basename "$DEBUG_CFG" | sed -E 's/^config-(.*)-debug$/\1/')   # e.g. 6.18.35-197
KATA_VER=$(kata-runtime --version 2>/dev/null | sed -nE 's/.*kata-runtime\s*:\s*([0-9.]+).*/\1/p' | head -1 || true)
[ -z "$KATA_VER" ] && { echo "ERROR: could not detect kata-runtime version (kata-runtime on PATH? $(command -v kata-runtime || echo no))" >&2; exit 1; }
DEV_KERNEL="$KATA_KDIR/vmlinux-${KVER}-research-dev"
HARD_KERNEL="$KATA_KDIR/vmlinux-${KVER}-research-hard"
log "kata $KATA_VER, guest kernel $KVER -> $DEV_KERNEL + $HARD_KERNEL"

# A vmlinux counts as "already built" if it exists AND carries BTF (the same
# silent-drop guard the ebpf script uses; KASAN presence is asserted at build).
has_btf() { [ -f "$1" ] && readelf -S "$1" 2>/dev/null | grep -q '\.BTF'; }

# --- shared instrumentation applied to BOTH variants ---
# KASAN (generic/SLUB, inline instrumentation) is the crash oracle; KCOV is
# coverage; BTF gives struct offsets; FPROBE/BSD_PROCESS_ACCT are the Tetragon
# substrate (as in kata-fc-ebpf); SYSVIPC/USER_NS/CONFIGFS are the reclaim-object
# surface (msg_msg, etc.); MODULES lets the researcher insmod the target driver.
apply_shared() {
  ./scripts/config --file .config \
    -e KASAN -e KASAN_GENERIC -e KASAN_INLINE \
    -e KCOV -e KCOV_INSTRUMENT_ALL \
    -e DEBUG_INFO_BTF \
    -e FPROBE -e BSD_PROCESS_ACCT \
    -e SYSVIPC -e USER_NS -e CONFIGFS_FS \
    -e MODULES -e MODULE_UNLOAD
}
# The flags whose survival through olddefconfig we assert (a dropped one = a
# broken lab: no oracle, no BTF, or no insmod).
SHARED_ASSERT="CONFIG_KASAN CONFIG_KCOV CONFIG_DEBUG_INFO_BTF CONFIG_FPROBE CONFIG_MODULES"

# dev: mitigations OFF - the environment to land a technique quickly.
apply_dev() {
  ./scripts/config --file .config \
    -d RANDOMIZE_BASE \
    -d SLAB_FREELIST_HARDENED \
    -d RANDOM_KMALLOC_CACHES
}
# hard: production-grade mitigations ON. NOTE: CONFIG_CFI_CLANG is deliberately
# NOT enabled here - it requires a clang/LLVM=1 kernel build (build-kernel.sh
# defaults to gcc), a larger change tracked as a fast-follow. The four flags
# below are the gcc-compatible mitigations and already make a genuinely hardened
# target (KASLR + hardened freelist + randomized caches + static usermodehelper).
apply_hard() {
  ./scripts/config --file .config \
    -e RANDOMIZE_BASE \
    -e SLAB_FREELIST_HARDENED \
    -e RANDOM_KMALLOC_CACHES \
    -e STATIC_USERMODEHELPER
}

# --- 1. build + install both kernels (skip a variant already present w/ BTF) ---
if has_btf "$DEV_KERNEL" && has_btf "$HARD_KERNEL"; then
  log "both research kernels already installed with BTF - skipping build"
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
  # Pristine post-setup baseline, so each variant starts from the same config
  # and the dev fragment's disables never leak into the hard build.
  cp .config ../.config.research-base

  # build_variant <apply_fragment_fn> <target_path> <label>
  build_variant() {
    local frag="$1" target="$2" label="$3"
    if has_btf "$target"; then log "$label kernel already installed - skipping"; return; fi
    log "configuring $label kernel (shared instrumentation + $label fragment)"
    cp ../.config.research-base .config
    apply_shared
    "$frag"
    make olddefconfig >/dev/null
    local f
    for f in $SHARED_ASSERT; do
      grep -q "^${f}=y" .config || { echo "ERROR: $f not enabled after olddefconfig for $label (pahole missing? modules unsupported?)" >&2; exit 1; }
    done
    log "compiling $label vmlinux ($(nproc) cores, ~20-30 min)"
    make -j"$(nproc)" vmlinux
    readelf -S vmlinux | grep -q '\.BTF' || { echo "ERROR: built $label vmlinux has no .BTF section" >&2; exit 1; }
    install -m 0644 vmlinux "$target"
    log "installed $target"
  }

  build_variant apply_dev  "$DEV_KERNEL"  "research-dev"
  build_variant apply_hard "$HARD_KERNEL" "research-hard"
fi

# --- 2. configuration-fc-research-{dev,hard}.toml = configuration-fc.toml + kernel ---
# Kata sizes the guest VM RAM from the pod's memory limit (SessionMemory=3072Mi
# for research, set in the backend), so no default_memory bump is needed here -
# same dynamic-sizing the ebpf runtime relies on. We only repoint the kernel.
write_toml() {
  local toml="$1" kernel="$2"
  log "writing $toml"
  cp "$KATA_DEFAULTS/configuration-fc.toml" "$toml"
  sed -i "s#^kernel = .*#kernel = \"$kernel\"#" "$toml"
  grep -q "^kernel = \"$kernel\"$" "$toml" || { echo "ERROR: kernel= not set in $toml" >&2; exit 1; }
}
write_toml "$KATA_DEFAULTS/configuration-fc-research-dev.toml"  "$DEV_KERNEL"
write_toml "$KATA_DEFAULTS/configuration-fc-research-hard.toml" "$HARD_KERNEL"

# --- 3. containerd handlers (both appended before a SINGLE restart) ---
NEED_RESTART=0
append_handler() {
  local name="$1" toml="$2"
  if grep -q "runtimes.${name}\]" "$CONTAINERD_CFG"; then
    log "containerd $name handler already present"
    return
  fi
  log "appending $name handler to $CONTAINERD_CFG"
  cat >> "$CONTAINERD_CFG" <<TOML

        [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.${name}]
  runtime_type = "io.containerd.kata.v2"
  snapshotter = "devmapper"
          [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.${name}.options]
    ConfigPath = "$toml"
TOML
  NEED_RESTART=1
}

if [ "$NEED_RESTART" = 0 ] && grep -q "runtimes.kata-fc-research-dev\]" "$CONTAINERD_CFG" && grep -q "runtimes.kata-fc-research-hard\]" "$CONTAINERD_CFG"; then
  log "both research handlers already present - no containerd change"
else
  cp "$CONTAINERD_CFG" "$CONTAINERD_CFG.bak.pre-research.$(date +%s)"
  append_handler kata-fc-research-dev  "$KATA_DEFAULTS/configuration-fc-research-dev.toml"
  append_handler kata-fc-research-hard "$KATA_DEFAULTS/configuration-fc-research-hard.toml"
fi

if [ "$NEED_RESTART" = 1 ]; then
  log "restarting containerd (running containers survive; brief CRI blip)"
  systemctl restart containerd
  sleep 4
  if systemctl is-active --quiet containerd && crictl info >/dev/null 2>&1; then
    for name in kata-fc-research-dev kata-fc-research-hard; do
      crictl info 2>/dev/null | grep -q "$name" && log "$name runtime registered" || echo "WARN: $name not visible in crictl info" >&2
    done
  else
    LATEST=$(ls -t "$CONTAINERD_CFG".bak.pre-research.* | head -1)
    echo "ERROR: containerd unhealthy after restart - restoring $LATEST" >&2
    cp "$LATEST" "$CONTAINERD_CFG"; systemctl restart containerd
    exit 1
  fi
fi

log "DONE. Apply the RuntimeClasses from the control plane if not already:"
log "  kubectl apply -f infra/runtimeclass/kata-fc-research-dev.yaml"
log "  kubectl apply -f infra/runtimeclass/kata-fc-research-hard.yaml"
