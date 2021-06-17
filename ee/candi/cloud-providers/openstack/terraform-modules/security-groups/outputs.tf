output "security_group_ids" {
  value = distinct(concat(data.openstack_networking_secgroup_v2.group[*].id, var.layout_security_group_ids))
}

output "security_group_names" {
  value = distinct(concat(data.openstack_networking_secgroup_v2.group[*].name, var.layout_security_group_names))
}
