# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "bastion_ip_address_for_ssh" {
  value = vcd_vapp_vm.bastion.network[0].ip
}
