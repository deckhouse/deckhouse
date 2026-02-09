# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "security_group_ids" {
  value = var.enabled ? [data.huaweicloud_networking_secgroup.kube[0].id] : []
}

output "security_group_names" {
  value = var.enabled ? [data.huaweicloud_networking_secgroup.kube[0].name] : []
}
