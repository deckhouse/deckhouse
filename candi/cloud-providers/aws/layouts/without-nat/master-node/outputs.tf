output "master_ip_address_for_ssh" {
  value = module.master-node.master_public_ip
}

output "node_internal_ip_address" {
  value = module.master-node.master_private_ip
}

output "kubernetes_data_device_path" {
  value = module.master-node.kubernetes_data_device_path
}
