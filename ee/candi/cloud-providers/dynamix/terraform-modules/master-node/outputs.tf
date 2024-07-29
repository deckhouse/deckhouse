# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  master_vm_network_extnet = [for net in tolist(decort_cb_kvmvm.master_vm.network): net if net.net_type == local.net_type_extnet]
  master_vm_network_vins = [for net in tolist(decort_cb_kvmvm.master_vm.network): net if net.net_type == local.net_type_vins]
}

output "master_ip_address_for_ssh" {
  value = local.master_vm_network_extnet[0].ip_address
}
output "node_internal_ip_address" {
  value = local.master_vm_network_vins[0].ip_address
}
output "kubernetes_data_device_path" {
  value = "/dev/vdb"
}
