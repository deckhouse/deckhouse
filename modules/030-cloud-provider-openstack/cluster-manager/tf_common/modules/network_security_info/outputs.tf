output "security_group_ids" {
  value = var.enabled ? [
    data.openstack_networking_secgroup_v2.default[0].id,
    data.openstack_networking_secgroup_v2.ssh_and_ping[0].id
  ] : []
}

output "security_group_names" {
  value = var.enabled ? [
    data.openstack_networking_secgroup_v2.default[0].name,
    data.openstack_networking_secgroup_v2.ssh_and_ping[0].name
  ] : []
}
