# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "id" {
  value = openstack_compute_instance_v2.master.id
}

output "node_internal_ip_address" {
  value = [for ip in openstack_compute_instance_v2.master.network: ip.fixed_ip_v4 if cidrhost("${ip.fixed_ip_v4}/${split("/", var.internal_network_cidr)[1]}", 0) == cidrhost(var.internal_network_cidr, 0)][0]
}

output "master_ip_address_for_ssh" {
  value = var.floating_ip_network == "" ? lookup(openstack_compute_instance_v2.master.network[0], "fixed_ip_v4") : openstack_compute_floatingip_v2.master[0].address
}
