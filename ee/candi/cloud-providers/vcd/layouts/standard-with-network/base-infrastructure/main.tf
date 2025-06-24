# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  useNSXT = var.providerClusterConfiguration.edgeGatewayType == "NSX-T"
}

module "vapp" {
  source                       = "../../../terraform-modules/vapp"
  providerClusterConfiguration = var.providerClusterConfiguration
}

module "vapp-network" {
  source                       = "../../../terraform-modules/vapp-network"
  providerClusterConfiguration = var.providerClusterConfiguration
}
