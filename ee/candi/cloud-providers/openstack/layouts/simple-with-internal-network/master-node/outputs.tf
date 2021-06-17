output "master_ip_address_for_ssh" {
  value = module.master.master_ip_address_for_ssh
}

output "node_internal_ip_address" {
  value = module.master.node_internal_ip_address
}

output "kubernetes_data_device_path" {
  value = module.kubernetes_data.device_path
}
