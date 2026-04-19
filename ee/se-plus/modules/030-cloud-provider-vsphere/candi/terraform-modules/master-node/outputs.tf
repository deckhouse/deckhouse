# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
locals {
  kubernetes_data_device_uuid_list = [
    for disk in vsphere_virtual_machine.master.disk :
    disk.uuid if disk.label == "disk1"
  ]
  kubernetes_data_device_uuid = replace(length(local.kubernetes_data_device_uuid_list) > 0 ? local.kubernetes_data_device_uuid_list[0] : "", "-", "")
  kubernetes_data_device_path = local.kubernetes_data_device_uuid != "" ? "/dev/disk/by-id/wwn-0x${local.kubernetes_data_device_uuid}" : "/dev/sdb"
}

output "master_ip_address_for_ssh" {
  value = vsphere_virtual_machine.master.default_ip_address
}

output "node_internal_ip_address" {
  value = length(local.additionalNetworks) == 0 ? vsphere_virtual_machine.master.default_ip_address : cidrhost(var.providerClusterConfiguration.internalNetworkCIDR, var.nodeIndex + 10)
}

output "kubernetes_data_device_path" {
  value = local.kubernetes_data_device_path
}

