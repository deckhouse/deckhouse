# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "huaweicloud_networking_secgroup" "kube" {
  count = var.enabled ? 1 : 0
  name = var.prefix
}
