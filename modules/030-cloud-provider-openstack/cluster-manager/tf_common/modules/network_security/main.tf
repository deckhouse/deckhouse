data "openstack_networking_secgroup_v2" "default" {
  count = var.enabled ? 1 : 0
  name = "default"
}

resource "openstack_networking_secgroup_v2" "ssh_and_ping" {
  count = var.enabled ? 1 : 0
  name = join("-", [var.prefix, "ssh-and-ping"])
}

resource "openstack_networking_secgroup_rule_v2" "allow_ssh" {
  count = var.enabled ? 1 : 0
  direction = "ingress"
  ethertype = "IPv4"
  protocol = "tcp"
  port_range_min = 22
  port_range_max = 22
  remote_ip_prefix = var.remote_ip_prefix
  security_group_id = openstack_networking_secgroup_v2.ssh_and_ping[0].id
}

resource "openstack_networking_secgroup_rule_v2" "allow_icmp" {
  count = var.enabled ? 1 : 0
  direction = "ingress"
  ethertype = "IPv4"
  protocol = "icmp"
  remote_ip_prefix = var.remote_ip_prefix
  security_group_id = openstack_networking_secgroup_v2.ssh_and_ping[0].id
}
