# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "master_ip_address_for_ssh" {
  value = module.master.master_ip_address_for_ssh
}

output "node_internal_ip_address" {
  value = module.master.node_internal_ip_address
}

output "kubernetes_data_device_path" {
  value = module.kubernetes_data.device_path
}
