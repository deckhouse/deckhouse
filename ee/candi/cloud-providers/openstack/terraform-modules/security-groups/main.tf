data "openstack_networking_secgroup_v2" "group" {
  count = length(var.security_group_names)
  name = element(var.security_group_names, count.index)
}
