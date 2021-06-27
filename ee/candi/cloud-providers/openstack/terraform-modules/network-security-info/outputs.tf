# Copyright 2021 Flant CJSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE

output "security_group_ids" {
  value = var.enabled ? [data.openstack_networking_secgroup_v2.kube[0].id] : []
}

output "security_group_names" {
  value = var.enabled ? [data.openstack_networking_secgroup_v2.kube[0].name] : []
}
