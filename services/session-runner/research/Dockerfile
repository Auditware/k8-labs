# syntax=docker/dockerfile:1

# The workstation image for the kernel-driver research labs (driver-research-dev
# / -hard). It carries a real kernel-exploitation toolchain, a planted-UAF target
# driver + starter fuzzer, AND the guest kernel's build tree - so the researcher
# can `make && insmod` the driver into the KASAN guest kernel in-session, which is
# the whole exploit path.
#
# glibc-based (Debian), NOT the Alpine foothold: the kernel build tree ships
# prebuilt host tools (fixdep, objtool, modpost) that must run in the workstation,
# and those are glibc binaries. A musl base could not run them.
#
# Baked into the session-runner airgap and imported into the nested cluster only
# for research sessions (entrypoint.sh gates research.tar on SESSION_RESEARCH=1),
# so no normal lab pays its weight. Referenced by internal/labs/modules/
# driver-research.yaml with imagePullPolicy: Never.

# TREE_MODE selects whether the (heavy, x86-only) kernel build tree is built:
#   full - CI/prod: build it, so `make` works in-session (the default).
#   none - local Tilt: skip it. The custom kernel is prod-only anyway (local labs
#          run runc on the host kernel), and an x86 tree can't be built on an
#          arm64 dev Mac. The Tiltfile passes TREE_MODE=none.
ARG TREE_MODE=full

# --- Stage 1: build the kernel build tree matching the on-box research kernel ---
# Reproduces the first half of infra/cluster/kata-fc-research-node.sh (same Kata
# version -> same 6.18.35 source), then applies the EXACT config captured from the
# running research kernel (kernel-6.18.35.config) and runs modules_prepare. Since
# the kernels are built with CONFIG_MODVERSIONS off, a modules_prepare tree with a
# matching config is enough to build loadable modules (only vermagic must match,
# and it does - same version + SMP + mod_unload). objtool is prebuilt because the
# kernel has CONFIG_OBJTOOL=y and external module builds invoke it.
FROM debian:bookworm-slim AS kbuild-full
ARG KATA_VER=3.32.0
ARG KVER=6.18.35
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential flex bison bc git curl ca-certificates zstd cpio kmod rsync \
      libelf-dev libssl-dev dwarves python3 \
 && curl -fsSL https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 \
      -o /usr/local/bin/yq && chmod +x /usr/local/bin/yq \
 && rm -rf /var/lib/apt/lists/*
WORKDIR /build
RUN git clone --depth 1 --branch "${KATA_VER}" https://github.com/kata-containers/kata-containers
WORKDIR /build/kata-containers/tools/packaging/kernel
RUN KERNEL_DEBUG_ENABLED=yes ./build-kernel.sh setup
COPY kernel-6.18.35.config /tmp/research.config
RUN set -eux; \
    KSRC="$(ls -d kata-linux-* | head -1)"; cd "$KSRC"; \
    cp /tmp/research.config .config; \
    make olddefconfig; \
    grep -q '^CONFIG_KASAN=y' .config; grep -q '^CONFIG_MODULES=y' .config; \
    make -j"$(nproc)" modules_prepare; \
    make -j"$(nproc)" scripts; \
    make -j"$(nproc)" tools/objtool; \
    # Assemble a linux-headers-style tree: headers + build infrastructure + the
    # built host tools, but NOT the kernel .c source (external module builds don't
    # need it) or other architectures - that keeps the tree to a few hundred MB.
    DEST="/out/lib/modules/${KVER}/build"; mkdir -p "$DEST"; \
    cp -a .config Makefile Module.symvers "$DEST"/ 2>/dev/null || true; \
    for d in scripts include arch/x86 tools/objtool tools/lib tools/include tools/arch tools/scripts; do \
      [ -e "$d" ] && { mkdir -p "$DEST/$d"; cp -a "$d/." "$DEST/$d/"; }; \
    done; \
    # Kbuild fragments that live at the source root
    find . -maxdepth 1 -name 'Kbuild*' -exec cp -a {} "$DEST"/ \; 2>/dev/null || true; \
    # Drop kernel .c source (keep host-tool sources under scripts/ and tools/) to
    # shrink; keep every header and build file.
    find "$DEST" -name '*.c' -not -path "$DEST/scripts/*" -not -path "$DEST/tools/*" -delete; \
    find "$DEST" -name '*.o' -not -path "$DEST/scripts/*" -not -path "$DEST/tools/*" -delete; \
    du -sh "$DEST"

# Local-only lightweight stand-in: an empty tree so the COPY below still resolves.
FROM debian:bookworm-slim AS kbuild-none
ARG KVER=6.18.35
RUN mkdir -p "/out/lib/modules/${KVER}/build"

# Pick the tree source by TREE_MODE (full in CI, none locally).
FROM kbuild-${TREE_MODE} AS kbuild

# --- Stage 2: the workstation image ---
FROM debian:bookworm-slim
ARG KVER=6.18.35
ENV DEBIAN_FRONTEND=noninteractive
# Runtime toolchain: gcc/make to build the driver, gdb, pahole (dwarves) for
# struct offsets, strace, ripgrep, kmod (insmod/rmmod/modinfo), plus bash/curl.
# elfutils/libssl/bison/flex are the kbuild runtime deps an external module build
# shells out to. Cleaned of apt lists to keep the layer lean.
RUN apt-get update && apt-get install -y --no-install-recommends \
      build-essential gdb dwarves strace ripgrep kmod bc \
      libelf1 libssl3 bison flex bash curl ca-certificates \
 && rm -rf /var/lib/apt/lists/*

# Point /bin/sh at bash. The terminal bridge (internal/labs/terminal.go) sets a
# bash-style PS1 (\u@\h, colors) and execs /bin/sh - written for the Alpine
# BusyBox foothold, whose ash interprets those escapes. Debian's default /bin/sh
# is dash, which prints them literally (an ugly raw prompt). bash renders them,
# and as `sh` it stays POSIX for the kbuild recipes.
RUN ln -sf /bin/bash /bin/sh

# The guest kernel's build tree, so `make` in /app/driver works in-session.
COPY --from=kbuild /out/lib/modules/${KVER}/build /lib/modules/${KVER}/build
# `make` looks up the tree via /lib/modules/$(uname -r)/build; source == build here.
RUN ln -sf /lib/modules/${KVER}/build /lib/modules/${KVER}/source

# The starter material: a sample planted-UAF driver (source + kbuild Makefile)
# and a starter ioctl fuzzer, as a starting point for driver-security research -
# not a CTF challenge, so nothing is left on disk to "read": all guidance lives in
# the in-app research guide. The workstation runs as root (the guest KERNEL is the
# target, not a uid boundary).
COPY app/ /app/

# research-driver: the in-guest helper that lists and loads the admin-vetted
# drivers a session mounts at /opt/drivers (see docs/architecture/research-
# driver-catalog.md). Make it executable (COPY may not preserve the bit) and put
# it on PATH so the researcher can just type `research-driver ls`.
RUN chmod +x /app/bin/research-driver \
 && ln -sf /app/bin/research-driver /usr/local/bin/research-driver

CMD ["sleep", "infinity"]
