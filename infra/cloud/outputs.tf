output "control_plane_ipv4" {
  description = "Public IPv4 of the control-plane node = CP_IP in kubeadm-2node.md, and the DNS target for k8labs.example.com."
  value       = hcloud_server.cp.ipv4_address
}

output "control_plane_ipv6" {
  value = hcloud_server.cp.ipv6_address
}
