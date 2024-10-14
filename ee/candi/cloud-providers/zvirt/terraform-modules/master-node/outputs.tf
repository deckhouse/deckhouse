# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "ovirt_wait_for_ip" "master_vm" {
  vm_id = ovirt_vm_start.master_vm.vm_id
}

locals {
  master_vm_interface = tolist(data.ovirt_wait_for_ip.master_vm.interfaces)[0]
  master_vm_ip = tolist(local.master_vm_interface.ipv4_addresses)[0]
}

output "master_ip_address_for_ssh" {
  value = local.master_vm_ip
}

output "node_internal_ip_address" {
  value = local.master_vm_ip
}

output "kubernetes_data_device_path" {
  value = "/dev/sdb"
}
