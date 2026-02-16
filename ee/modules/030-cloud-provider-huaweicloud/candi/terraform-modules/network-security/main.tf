# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

resource "huaweicloud_networking_secgroup" "kube" {
  count = var.enabled ? 1 : 0
  name = var.prefix
  enterprise_project_id = var.enterprise_project_id
}

resource "huaweicloud_networking_secgroup_rule" "allow_ssh" {
  count = var.enabled ? length(var.ssh_allow_list) : 0
  direction = "ingress"
  ethertype = "IPv4"
  protocol = "tcp"
  port_range_min = 22
  port_range_max = 22
  remote_ip_prefix = var.ssh_allow_list[count.index]
  security_group_id = huaweicloud_networking_secgroup.kube[0].id
}

resource "huaweicloud_networking_secgroup_rule" "allow_icmp" {
  count = var.enabled ? 1 : 0
  direction = "ingress"
  ethertype = "IPv4"
  protocol = "icmp"
  remote_ip_prefix = "0.0.0.0/0"
  security_group_id = huaweicloud_networking_secgroup.kube[0].id
}

resource "huaweicloud_networking_secgroup_rule" "allow_node_ports" {
  count = var.enabled ? 1 : 0
  direction = "ingress"
  ethertype = "IPv4"
  protocol = "tcp"
  port_range_min = 30000
  port_range_max = 32767
  remote_ip_prefix = "0.0.0.0/0"
  security_group_id = huaweicloud_networking_secgroup.kube[0].id
  description = "Allow access to node ports"
}

# resource "huaweicloud_networking_secgroup_rule" "allow_internal_communication" {
#   count = var.enabled ? 1 : 0
#   direction = "ingress"
#   ethertype = "IPv4"
#   security_group_id = huaweicloud_networking_secgroup.kube[0].id
#   remote_group_id = huaweicloud_networking_secgroup.kube[0].id
#   description = "Allow internal communication between nodes"
# }
