# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  useNSXT       = var.providerClusterConfiguration.edgeGateway.type == "NSX-T"
  edgeGatewayId = local.useNSXT ? data.vcd_nsxt_edgegateway.gateway[0].id : data.vcd_edgegateway.gateway[0].id
}

data "vcd_nsxt_edgegateway" "gateway" {
  count = local.useNSXT ? 1 : 0
  org   = var.providerClusterConfiguration.organization
  name  = var.providerClusterConfiguration.edgeGateway.name
}

data "vcd_edgegateway" "gateway" {
  count = local.useNSXT ? 0 : 1
  org   = var.providerClusterConfiguration.organization
  name  = var.providerClusterConfiguration.edgeGateway.name
}

module "network" {
  source                       = "../../../terraform-modules/network"
  providerClusterConfiguration = var.providerClusterConfiguration
  edgeGatewayId = local.edgeGatewayId
  useNSXT = local.useNSXT
}

module "vapp" {
  source                       = "../../../terraform-modules/vapp"
  providerClusterConfiguration = var.providerClusterConfiguration
}

resource "vcd_vapp_org_network" "vapp_network" {
  org                    = var.providerClusterConfiguration.organization
  vdc                    = var.providerClusterConfiguration.virtualDataCenter
  vapp_name              = module.vapp.name
  org_network_name       = module.network.name
  reboot_vapp_on_removal = true
}

module "nat" {
  source                       = "../../../terraform-modules/nat"
  providerClusterConfiguration = var.providerClusterConfiguration
  edgeGatewayId               = local.edgeGatewayId
  useNSXT                     = local.useNSXT
  depends_on                  = [vcd_vapp_org_network.vapp_network]
}
