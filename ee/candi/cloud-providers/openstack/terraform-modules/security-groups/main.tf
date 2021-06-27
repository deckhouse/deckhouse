# Copyright 2021 Flant CJSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE

data "openstack_networking_secgroup_v2" "group" {
  count = length(var.security_group_names)
  name = element(var.security_group_names, count.index)
}
