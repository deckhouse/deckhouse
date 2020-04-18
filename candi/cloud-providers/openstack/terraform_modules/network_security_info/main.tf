data "openstack_networking_secgroup_v2" "default" {
  count = var.enabled ? 1 : 0
  name = "default"
}

data "openstack_networking_secgroup_v2" "ssh_and_ping" {
  count = var.enabled ? 1 : 0
  name = join("-", [var.prefix, "ssh-and-ping"])
}
