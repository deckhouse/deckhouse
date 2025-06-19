# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

resource "vcd_vapp" "vapp" {
  name     = var.providerClusterConfiguration.virtualApplicationName
  org      = var.providerClusterConfiguration.organization
  power_on = true
}
