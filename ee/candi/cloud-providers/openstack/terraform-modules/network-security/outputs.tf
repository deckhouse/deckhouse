output "security_group_names" {
  value = var.enabled ? [openstack_networking_secgroup_v2.kube[0].name] : []
}
