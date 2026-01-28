# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "master_ip_address_for_ssh" {
  value = vcd_vapp_vm.master.network[0].ip
}

output "node_internal_ip_address" {
  value = vcd_vapp_vm.master.network[0].ip
}

output "kubernetes_data_device_path" {
  value = "/dev/sdb"
}
