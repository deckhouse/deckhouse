# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  network_list = tolist(decort_kvmvm.master_vm.interfaces)
  master_vm_network_extnet = [for net in local.network_list: net if net.net_type == local.net_type_extnet]
  master_vm_network_vins = [for net in local.network_list: net if net.net_type == local.net_type_vins]
  extnet_ip = local.master_vm_network_extnet[0].ip_address
  vins_ip = length(local.master_vm_network_vins) > 0 ? local.master_vm_network_vins[0].ip_address : local.extnet_ip
}

output "master_ip_address_for_ssh" {
  value = local.extnet_ip
}

output "node_internal_ip_address" {
  value = local.vins_ip
}

output "kubernetes_data_device_path" {
  value = "/dev/vdb"
}
