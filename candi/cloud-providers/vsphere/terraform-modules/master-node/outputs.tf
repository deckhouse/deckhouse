output "master_ip_address_for_ssh" {
  value = vsphere_virtual_machine.master.default_ip_address
}

output "node_internal_ip_address" {
  value = length(local.additionalNetworks) == 0 ? vsphere_virtual_machine.master.default_ip_address : cidrhost(var.providerClusterConfiguration.internalNetworkCIDR, var.nodeIndex + 10)
}

output "kubernetes_data_device_path" {
  value = "/dev/sdb"
}
