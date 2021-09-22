# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "id" {
  value = openstack_compute_instance_v2.master.id
}

output "node_internal_ip_address" {
  value = lookup(openstack_compute_instance_v2.master.network[0], "fixed_ip_v4")
}

output "master_ip_address_for_ssh" {
  value = var.floating_ip_network == "" ? lookup(openstack_compute_instance_v2.master.network[0], "fixed_ip_v4") : openstack_compute_floatingip_v2.master[0].address
}
