# Path B - kubeadm 2-node bring-up

Produces the same cluster as CAPH (see `prod-components.svg`), by hand, in
about an hour. Two nodes: a Hetzner **Cloud CX33** (control plane + platform,
runc, untainted) and the **bare-metal AX41** (labs only, Kata/Firecracker,
tainted). After this, `bootstrap.sh` + `docs/self-hosting.md` take over - those are
path-agnostic.

Pin one Kubernetes version everywhere. This runbook uses **v1.31**.

## 0. Networking decision (read first)

The two nodes talk over their **public IPs** (the AX41 is a Robot dedicated
server; wiring it into a Hetzner Cloud private network needs a vSwitch, which
is the fiddly bit Path B exists to avoid). To keep cross-node pod traffic off
the clear internet, **Cilium encrypts it with WireGuard** (step 3), and a
Hetzner Cloud firewall + the AX41's own firewall lock the node ports down to
each other + your admin IP. vSwitch is the cleaner upgrade later; not needed
for v1.

## 1. Create the control-plane node (Hetzner Cloud console)

New project → new server: **CX33**, **NBG1** (the CX Cost-Optimized line isn't
stocked in fsn1; nbg1 is the closest park - matches prod), **Ubuntu 24.04**, attach your
SSH key. Note its public IPv4 (call it `CP_IP` below). Nothing else.

## 2. Control plane on the CX33

SSH in as root, then:

```bash
# --- container runtime (containerd) ---
apt-get update && apt-get install -y containerd apt-transport-https ca-certificates curl gpg
mkdir -p /etc/containerd && containerd config default > /etc/containerd/config.toml
sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
systemctl restart containerd

# --- kubeadm/kubelet/kubectl (v1.31) ---
curl -fsSL https://pkgs.k8s.io/core:/stable:/v1.31/deb/Release.key \
  | gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v1.31/deb/ /" \
  > /etc/apt/sources.list.d/kubernetes.list
apt-get update && apt-get install -y kubelet kubeadm kubectl && apt-mark hold kubelet kubeadm kubectl
swapoff -a   # kubelet requires swap off

# --- init the control plane. --skip-phases=addon/kube-proxy: Cilium replaces
# kube-proxy (step 3). Pod CIDR is Cilium's default. ---
kubeadm init \
  --control-plane-endpoint "CP_IP" \
  --pod-network-cidr "10.0.0.0/8" \
  --skip-phases=addon/kube-proxy

# kubeconfig for this shell (and copy to your laptop's ~/.kube/config)
export KUBECONFIG=/etc/kubernetes/admin.conf

# untaint so platform pods schedule on this single node
kubectl taint nodes --all node-role.kubernetes.io/control-plane-
```

Save the `kubeadm join ...` command it prints - you need it in step 4. (Regenerate
later with `kubeadm token create --print-join-command`.)

## 3. Cilium (CNI) - kube-proxy replacement + WireGuard

```bash
helm repo add cilium https://helm.cilium.io/ && helm repo update cilium
helm install cilium cilium/cilium --version 1.16.5 --namespace kube-system \
  --set kubeProxyReplacement=true \
  --set k8sServiceHost=CP_IP --set k8sServicePort=6443 \
  --set ipam.mode=kubernetes --set operator.replicas=1 \
  --set encryption.enabled=true --set encryption.type=wireguard
cilium status --wait
```

> This supersedes `bootstrap.sh`'s step [1/8] Cilium - it's the same chart with
> Path-B networking flags. Run Cilium here, then in `bootstrap.sh` **skip its
> Cilium step** (or match these flags) so it doesn't overwrite the WireGuard
> config. Everything else in `bootstrap.sh` runs as-is.

The control-plane node should now be `Ready`.

## 4. Join the bare-metal labs node (AX41)

The AX41 is in the rescue system. Install a real OS, then join.

```bash
# --- from the AX41 rescue shell: lay down Ubuntu 24.04 ---
installimage    # interactive: pick Ubuntu 24.04, confirm the RAID1/partitions
reboot          # boots into the installed OS

# --- back on the AX41 (now Ubuntu): containerd + kata + kube tools ---
# containerd + kube v1.31: same commands as step 2 (runtime + kubelet/kubeadm),
# WITHOUT kubectl/init - this is a worker.

# Kata Containers + Firecracker. NOTE: kata-deploy alone is NOT enough for
# kata-fc - Firecracker needs a block-backed rootfs via the containerd
# `devmapper` snapshotter, which kata-deploy does not set up. Verified working
# steps on the AX41 (A3, see docs/architecture/nested-lab-rewrite.md):

# 1) kata-static + firecracker to /opt/kata, shim on PATH
VER=$(curl -s https://api.github.com/repos/kata-containers/kata-containers/releases/latest | grep -m1 tag_name | cut -d'"' -f4)
curl -sSL -o /tmp/kata.tar.zst "https://github.com/kata-containers/kata-containers/releases/download/${VER}/kata-static-${VER}-amd64.tar.zst"
tar --zstd -xf /tmp/kata.tar.zst -C /
ln -sf /opt/kata/bin/containerd-shim-kata-v2 /usr/local/bin/
# privileged pods break under kata (host block-device hotplug); the runner uses
# capabilities instead, so no config change needed there.

# 2) devmapper thinpool. TEST used a loopback pool (below); PROD should back it
#    with a real LV/disk + a systemd unit that recreates the dm target on boot.
DD=/var/lib/containerd-devmapper; mkdir -p $DD
truncate -s 40G $DD/data; truncate -s 4G $DD/meta
DATA=$(losetup --find --show $DD/data); META=$(losetup --find --show $DD/meta)
dmsetup create devpool --table "0 $(( $(blockdev --getsize64 -q $DATA)/512 )) thin-pool $META $DATA 128 32768 1 skip_block_zeroing"

# 3) containerd config (kubeadm containerd = /etc/containerd/config.toml, v3):
#    add the devmapper snapshotter plugin + a kata-fc runtime that USES it:
#    [plugins.'io.containerd.snapshotter.v1.devmapper']
#      pool_name="devpool"; root_path="/var/lib/containerd-devmapper"
#      base_image_size="10GB"; discard_blocks=true
#    [plugins.'io.containerd.cri.v1.runtime'.containerd.runtimes.kata-fc]
#      runtime_type="io.containerd.kata.v2"; snapshotter="devmapper"
#      [...kata-fc.options] ConfigPath="/opt/kata/share/defaults/kata-containers/configuration-fc.toml"
#    then: systemctl restart containerd
# 4) RuntimeClass: kubectl apply a RuntimeClass{name: kata-fc, handler: kata-fc}
# (see infra/runtimeclass/kata-fc.yaml)

# --- join the cluster (the command saved from step 2) ---
kubeadm join CP_IP:6443 --token <...> --discovery-token-ca-cert-hash sha256:<...>
```

Then, from your admin kubeconfig, label + taint the labs node and confirm the
RuntimeClass exists:

```bash
kubectl label node <ax41-node-name> k8labs.io/role=labs
# taint so ONLY lab pods (which tolerate it) land here - keeps the hostile
# node clear of platform pods. NOTE: lab pod specs need a matching toleration
# added in internal/labs (see "code change" below).
kubectl taint node <ax41-node-name> k8labs.io/dedicated=labs:NoSchedule

# kata-deploy makes the kata-fc RuntimeClass; if you want the node-scoped
# variant instead, apply ours:
kubectl apply -f infra/runtimeclass/kata-fc.yaml
```

### kata-fc-ebpf (opt-in eBPF labs) - a delta on the above

eBPF lab modules (`Module.EBPF`, e.g. `ebpf-detect`) need a guest kernel with
BTF + kprobes + tracefs so a student can run Tetragon INSIDE their microVM. This
is a per-session opt-in on a SEPARATE RuntimeClass so the default `kata-fc` keeps
its lean, fast-booting kernel. It reuses kata-fc's /opt/kata + devmapper - only
adding a guest kernel and a second runtime handler. The shipped -debug kernel is
NOT sufficient (the Phase-4 gate found it lacks kprobe_multi + acct_process, and
BTF is silently dropped without pahole - full recipe in
`docs/architecture/ebpf-per-session.md`), so a custom kernel is built.

The whole node-side setup is scripted and idempotent - run it on the labs node
(the AX41) after kata-fc works. It detects the kata + kernel versions, builds the
kernel (~20-30 min the first time, skipped on re-run), writes
`configuration-fc-ebpf.toml`, and adds the `kata-fc-ebpf` containerd handler:

```bash
# On the AX41 (copy the repo script over, or run from a checkout):
sudo bash infra/cluster/kata-fc-ebpf-node.sh
```

Then, from the control plane, the cluster-level RuntimeClass:

```bash
kubectl apply -f infra/runtimeclass/kata-fc-ebpf.yaml
```

The backend then uses it automatically for eBPF modules
(`config.labsRuntimeClassEbpf=kata-fc-ebpf`, the chart default; the in-guest
sensor's kallsyms sysctls are set by the session-runner entrypoint). Phase-4 gate
before granting an eBPF lab: confirm a real CO-RE Tetragon sensor LOADS in-guest
(boot an `ebpf-detect` session and check the sensor logs "Listening for
events") - PASSED on the AX41 2026-07, see the design doc.

### kata-fc-research-{dev,hard} (grant-only kernel-exploitation labs) - a delta on the above

The kernel-driver research labs (`driver-research-dev` / `-hard`) go a step
past eBPF: the guest KERNEL itself is the exploitation target. They need a
KASAN + KCOV + BTF instrumented kernel (KASAN is the crash oracle a memory-safety
exploit is built around) with loadable-module support so the researcher can
`insmod` the sample driver. Two kernels are built from one instrumented base,
differing only in a mitigation fragment: `research-dev` (mitigations off, land
the technique) and `research-hard` (KASLR + hardened freelist + randomized
kmalloc caches + static usermodehelper on, prove it survives). Same reuse of
kata-fc's /opt/kata + devmapper as eBPF; each is its own RuntimeClass, and the
labs are grant-only so concurrency (and the heavy KASAN RAM) stays low. Full
design: `docs/architecture/kasan-research-lab.md`.

The node-side setup is scripted and idempotent (same shape as the eBPF script):
it detects the kata + kernel versions, builds BOTH kernels (~20-30 min each the
first time, skipped on re-run), writes `configuration-fc-research-{dev,hard}
.toml`, and appends both containerd handlers before a single restart (with
backup + health-check + auto-rollback):

```bash
# On the AX41 (copy the repo script over, or run from a checkout):
sudo bash infra/cluster/kata-fc-research-node.sh
```

Then, from the control plane, the two cluster-level RuntimeClasses:

```bash
kubectl apply -f infra/runtimeclass/kata-fc-research-dev.yaml
kubectl apply -f infra/runtimeclass/kata-fc-research-hard.yaml
```

The backend uses them automatically for the research modules
(`config.labsRuntimeClassResearchDev/Hard`, the chart defaults; the research
workstation image is imported in-guest by the session-runner entrypoint when the
backend sets `SESSION_RESEARCH=1`). `CONFIG_CFI_CLANG` on the hardened kernel is
a documented fast-follow (it needs a clang/LLVM build). Acceptance before
granting: boot a `driver-research-dev` session, `make && insmod kuaf.ko` in
`/app/driver`, run `/app/fuzz/fuzz uaf`, and confirm a KASAN report in `dmesg` +
the `commit_creds` event in the Defender pane.

## 5. Hand off to the platform (path-agnostic)

```bash
source secrets/argocd.env          # GITHUB_PAT
./infra/cluster/bootstrap.sh       # SKIP its Cilium step (already done, step 3)
```

Then the secret-valued + external steps in `docs/self-hosting.md`:
Vault init/unseal + KV, GHCR pull secret, cert-manager ACME issuer, the
`k8labs.example.com` DNS A record → `CP_IP`, OAuth redirect registration.

## Code change this path implies (not yet done)

The node taint in step 4 keeps platform pods off the labs node - but lab pods
must **tolerate** it, or they won't schedule there either. Add a toleration for
`k8labs.io/dedicated=labs:NoSchedule` to the lab pod specs in
`internal/labs/session.go` (`buildPrimaryPod` / `buildTargetPod`), alongside
the existing `nodeSelector`. Without the taint the setup still works (pods land
on the labs node via nodeSelector); the taint is the hardening that guarantees
nothing else does. Do it before relying on the isolation.
