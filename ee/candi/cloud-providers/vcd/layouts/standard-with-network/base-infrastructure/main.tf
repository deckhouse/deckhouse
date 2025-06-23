# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  useNSXT = var.providerClusterConfiguration.edgeGatewayType == "NSX-T"
}

module "vapp" {
  source                       = "../../../terraform-modules/vapp"
  providerClusterConfiguration = var.providerClusterConfiguration
}

module "network" {
  source                       = "../../../terraform-modules/network"
  providerClusterConfiguration = var.providerClusterConfiguration
}

resource "vcd_vapp_org_network" "vapp_network" {
  org                    = var.providerClusterConfiguration.organization
  vdc                    = var.providerClusterConfiguration.virtualDataCenter
  vapp_name              = module.vapp.name
  org_network_name       = module.network.name
  reboot_vapp_on_removal = true
}
