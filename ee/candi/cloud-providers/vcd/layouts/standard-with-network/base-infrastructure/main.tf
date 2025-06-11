# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

module "network" {
  source                       = "../../../terraform-modules/network"
  providerClusterConfiguration = var.providerClusterConfiguration
  prefix                       = var.clusterConfiguration.prefix
}

module "vapp" {
  source                       = "../../../terraform-modules/vapp"
  prefix                       = var.clusterConfiguration.prefix
  providerClusterConfiguration = var.providerClusterConfiguration
}

resource "vcd_vapp_org_network" "vapp_network" {
  org              = var.providerClusterConfiguration.organization
  vdc              = var.providerClusterConfiguration.virtualDataCenter
  vapp_name        = var.clusterConfiguration.prefix
  org_network_name = var.clusterConfiguration.prefix
}
