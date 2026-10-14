# Path B control-plane node as code. Provisions the single Hetzner Cloud VM
# that runs the k8s control plane + all platform pods (the AX41 bare-metal
# labs node is Robot-only - no create API - so it stays out of Terraform).
#
# Auth: the hcloud provider reads HCLOUD_TOKEN from the environment. Never put
# the token in a .tf or committed .tfvars - `source secrets/hcloud.env` first.
#   source secrets/hcloud.env && terraform apply

terraform {
  required_version = ">= 1.5"
  required_providers {
    hcloud = {
      source  = "hetznercloud/hcloud"
      version = "~> 1.48"
    }
  }
}

provider "hcloud" {}

# Uploads the k8labs SSH public key so the server trusts it. Public key only -
# the private half never leaves secrets/ (git-secret) / your password manager.
resource "hcloud_ssh_key" "k8labs" {
  name       = "k8labs"
  public_key = var.ssh_public_key
}

# Firewall: public gets only the ingress (80/443) and, from your admin IP,
# SSH + the kube API. The labs node (AX41) gets full node-to-node reachability
# so Cilium (incl. WireGuard) works across the two machines.
resource "hcloud_firewall" "cp" {
  name = "k8labs-cp"

  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = [var.admin_cidr]
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "6443"
    source_ips = [var.admin_cidr, var.labs_node_cidr]
  }
  # 80/443 only from Cloudflare's edge - the site is reachable ONLY through the
  # proxy, so no one bypasses the WAF/SSL by hitting the origin IP directly.
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = var.cloudflare_cidrs
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = var.cloudflare_cidrs
  }
  # Node-to-node (AX41 ↔ this node): allow all tcp/udp/icmp so Cilium's
  # datapath + WireGuard (UDP 51871) work without enumerating every port.
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "any"
    source_ips = [var.labs_node_cidr]
  }
  rule {
    direction  = "in"
    protocol   = "udp"
    port       = "any"
    source_ips = [var.labs_node_cidr]
  }
  rule {
    direction  = "in"
    protocol   = "icmp"
    source_ips = [var.labs_node_cidr]
  }
}

resource "hcloud_server" "cp" {
  name         = "k8labs-cp"
  server_type  = var.server_type # cx33 = 4 vCPU / 8 GB (Intel Cost-Optimized)
  image        = var.image
  location     = var.location # nbg1 - closest CX-stocked park to the AX41 (fsn1)
  ssh_keys     = [hcloud_ssh_key.k8labs.name]
  firewall_ids = [hcloud_firewall.cp.id]

  public_net {
    ipv4_enabled = true
    ipv6_enabled = true
  }

  labels = {
    project = "k8labs"
    role    = "control-plane"
  }
}
