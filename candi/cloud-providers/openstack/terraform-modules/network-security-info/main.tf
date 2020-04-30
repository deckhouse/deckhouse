data "openstack_networking_secgroup_v2" "kube" {
  count = var.enabled ? 1 : 0
  name = var.prefix
}
