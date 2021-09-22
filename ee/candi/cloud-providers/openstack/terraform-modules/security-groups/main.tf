# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "openstack_networking_secgroup_v2" "group" {
  count = length(var.security_group_names)
  name = element(var.security_group_names, count.index)
}
