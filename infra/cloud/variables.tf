# The SSH *public* key - not a secret (public by design), so it can live inline
# here, no file/env-var/git-secret needed (that's only for the token). No key
# ships with the repo: supply your own via terraform.tfvars or
# `export TF_VAR_ssh_public_key=...`.
variable "ssh_public_key" {
  description = "SSH public key uploaded to the server - supply your own (e.g. ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAExampleReplaceWithYourOwnKey your-key)."
  type        = string
  default     = ""
}

variable "admin_cidr" {
  description = "Your admin IP in CIDR form (e.g. 203.0.113.7/32) - the only source allowed to SSH and reach the kube API. Set this; do not leave it open."
  type        = string
}

variable "labs_node_cidr" {
  description = "The AX41 labs node's public IP in CIDR form - gets full node-to-node reachability for Cilium."
  type        = string
  default     = "203.0.113.10/32"
}

# Cloudflare's published edge IP ranges (https://www.cloudflare.com/ips/).
# The origin's 80/443 are restricted to these so the site can ONLY be reached
# through the Cloudflare proxy - a direct hit on the origin IP is dropped, which
# stops anyone bypassing the WAF/rate-limits/SSL by hitting the IP. Refresh from
# the URL above if Cloudflare changes ranges (rare).
variable "cloudflare_cidrs" {
  type = list(string)
  default = [
    "173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
    "141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
    "197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
    "104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
    "2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
    "2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
  ]
}

variable "server_type" {
  description = "Hetzner Cloud server type. cx33 = 4 vCPU / 8 GB (Intel Cost-Optimized line, EU-only: nbg1/hel1/fsn1), the combined control-plane + platform node for the private beta. Resize up to cx43 (8/16) in place when real load arrives."
  type        = string
  default     = "cx33"
}

variable "image" {
  type    = string
  default = "ubuntu-24.04"
}

# Nuremberg, not Falkenstein: the AX41 labs node is in Falkenstein (fsn1), but the
# Cost-Optimized CX line isn't stocked there - nbg1 is the closest park that has it
# (both central Germany, ~3-6ms to fsn1). CP<->worker runs over public IP + Cilium
# WireGuard, so different-park is fine; nbg1 just keeps cross-node latency near zero.
variable "location" {
  type    = string
  default = "nbg1"
}
