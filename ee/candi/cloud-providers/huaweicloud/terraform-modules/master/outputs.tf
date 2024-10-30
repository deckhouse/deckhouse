# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "id" {
  value = huaweicloud_compute_instance.master.id
}

output "node_internal_ip_address" {
  value = [for ip in huaweicloud_compute_instance.master.network: ip.fixed_ip_v4 if cidrhost("${ip.fixed_ip_v4}/${split("/", var.internal_network_cidr)[1]}", 0) == cidrhost(var.internal_network_cidr, 0)][0]
}

output "master_ip_address_for_ssh" {
  value = var.enable_eip == true ? huaweicloud_vpc_eip.master[0].address : lookup(huaweicloud_compute_instance.master.network[0], "fixed_ip_v4")
}
