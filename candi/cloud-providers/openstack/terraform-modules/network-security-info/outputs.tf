output "security_group_ids" {
  value = var.enabled ? [data.openstack_networking_secgroup_v2.kube[0].id] : []
}

output "security_group_names" {
  value = var.enabled ? [data.openstack_networking_secgroup_v2.kube[0].name] : []
}
