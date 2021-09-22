# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "security_group_names" {
  value = var.enabled ? [openstack_networking_secgroup_v2.kube[0].name] : []
}
