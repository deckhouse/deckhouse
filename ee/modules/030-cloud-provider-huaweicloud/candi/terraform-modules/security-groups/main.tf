# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "huaweicloud_networking_secgroup" "group" {
  count = length(var.security_group_names)
  name = element(var.security_group_names, count.index)
  enterprise_project_id = var.enterprise_project_id
}
