# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "master_ip_address_for_ssh" {
  value = ovirt_wait_for_ip.master_wm.interfaces.ipv4_addresses
}

output "node_internal_ip_address" {
  value = ovirt_wait_for_ip.master_wm.interfaces.ipv4_addresses
}

output "kubernetes_data_device_path" {
  value = "/dev/sdb"
}
