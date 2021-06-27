# Copyright 2021 Flant CJSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE

output "security_group_ids" {
  value = distinct(concat(data.openstack_networking_secgroup_v2.group[*].id, var.layout_security_group_ids))
}

output "security_group_names" {
  value = distinct(concat(data.openstack_networking_secgroup_v2.group[*].name, var.layout_security_group_names))
}
