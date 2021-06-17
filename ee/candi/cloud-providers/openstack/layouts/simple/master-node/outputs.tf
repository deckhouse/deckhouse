output "master_ip_address_for_ssh" {
  value = openstack_compute_instance_v2.master.network[0].fixed_ip_v4
}

output "node_internal_ip_address" {
  value = openstack_compute_instance_v2.master.network[0].fixed_ip_v4
}

output "kubernetes_data_device_path" {
  value = module.kubernetes_data.device_path
}
