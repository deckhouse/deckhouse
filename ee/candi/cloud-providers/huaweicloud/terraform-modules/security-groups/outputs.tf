# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "security_group_ids" {
  value = distinct(concat(data.huaweicloud_networking_secgroup.group[*].id, var.layout_security_group_ids))
}

output "security_group_names" {
  value = distinct(concat(data.huaweicloud_networking_secgroup.group[*].name, var.layout_security_group_names))
}
