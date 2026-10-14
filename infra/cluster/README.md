# infra/cluster - production cluster bootstrap (Path A: CAPH)

> **What actually runs prod:** the live cluster was built by hand via **Path B**
> (`infra/cluster/kubeadm-2node.md` + Terraform in `infra/cloud/`), NOT CAPH. See
> `docs/self-hosting.md` for the deployed nodes. This directory
> documents **Path A** - the Cluster-API / CAPH bootstrap - kept as the automated
> alternative to pivot to later; it is not what is deployed today.

Path A provisions the cluster with **Cluster API + Cluster API Provider
Hetzner (CAPH)**. This is a one-time bootstrap; day-2 app delivery is ArgoCD
(see `apps/**` and `argocd/**`), not this directory.

Topology this produces (see `docs/architecture/prod-components.svg`):

- **Control plane + platform**: Hetzner **Cloud** VMs (`hcloud`), plain runc.
  For a cheap v1: a single `CX33` control-plane node (resize to `CX43` under
  load), untainted so platform
  pods (backend, postgres, vault, argocd, observability…) schedule on it too.
- **Labs**: a Hetzner **bare-metal** machine (Robot, e.g. AX41) joined as a
  worker, labelled `k8labs.io/role=labs`, running the `kata-fc` RuntimeClass
  (`infra/runtimeclass/kata-fc.yaml`) so lab pods get microVM isolation.

## Why the two-machine split is a CAPH mix

Cloud nodes come from `HCloudMachineTemplate` (API-driven, fully automated).
The bare-metal AX41 has **no creation API** - you order it manually in the
Hetzner Robot console once, then CAPH adopts it via `HetznerBareMetalHost` +
`HetznerBareMetalMachineTemplate` and provisions the OS over its rescue
system. So the AX41 order is a manual prerequisite; everything after is
declarative.

## Bootstrap flow

1. **Order the AX41** in Robot (manual, one-off). Note its server number and
   set a rescue-system root password.
2. **Temporary management cluster** - CAPH bootstraps from a throwaway local
   `kind` cluster, then pivots into the workload cluster itself (no permanent
   separate management cluster):
   ```
   kind create cluster --name capi-mgmt
   clusterctl init --infrastructure hetzner
   ```
3. **Supply variables** - copy `clusterctl-vars.env.example` to
   `secrets/clusterctl-vars.env` (git-secret encrypted, never plaintext in
   git - see `secrets/README.md`), fill in the real token/robot creds, and
   `source` it.
4. **Generate + apply** the cluster manifests from CAPH's own template
   (do not hand-write the CRDs - the provider's template tracks its CRD
   schema per version):
   ```
   clusterctl generate cluster k8labs \
     --infrastructure hetzner \
     --kubernetes-version v1.31.x \
     --control-plane-machine-count 1 \
     --worker-machine-count 0 > cluster.yaml
   # add the bare-metal MachineDeployment + HetznerBareMetalHost for the AX41
   # (labs worker) - see CAPH docs "hetzner-baremetal"; it is not part of the
   # default hcloud template.
   kubectl apply -f cluster.yaml
   ```
5. **Wait + fetch kubeconfig**:
   ```
   clusterctl describe cluster k8labs        # watch until control plane is Ready
   clusterctl get kubeconfig k8labs > secrets/k8labs.kubeconfig
   ```
6. **Pivot** the CAPI/CAPH controllers into the workload cluster, then delete
   the kind cluster:
   ```
   clusterctl init --infrastructure hetzner --kubeconfig secrets/k8labs.kubeconfig
   clusterctl move --to-kubeconfig secrets/k8labs.kubeconfig
   kind delete cluster --name capi-mgmt
   ```
7. **Label the labs node + install its RuntimeClass**:
   ```
   kubectl label node <ax41-node> k8labs.io/role=labs
   # kata-deploy installs the kata-fc containerd handler on that node, then:
   kubectl apply -f infra/runtimeclass/kata-fc.yaml
   ```
8. **Run the platform bootstrap** - one command installs the substrate
   (Cilium, ArgoCD, cert-manager, Vault, Tetragon, observability) and hands the
   app tier to ArgoCD's App-of-Apps:
   ```
   source secrets/argocd.env    # for GITHUB_PAT (private-repo read cred)
   ./infra/cluster/bootstrap.sh
   ```
   Then the secret-valued + external steps (Vault init/unseal, GHCR pull
   secret, ACME issuer, DNS, OAuth) - see `docs/self-hosting.md`,
   which is where prod stops being "provision infra" and becomes "deploy the
   platform."

## What is NOT here yet (blocking)

- No real `cluster.yaml` is committed - it's generated from the CAPH template
  at bootstrap time with real values, and the bare-metal worker block still
  needs authoring against the CAPH baremetal CRDs.
- The `kata-fc` containerd handler / kata-deploy step on the AX41 is unbuilt.
- Hetzner project, `HCLOUD_TOKEN`, and Robot credentials don't exist yet.
