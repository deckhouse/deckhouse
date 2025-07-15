# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

module "master_node" {
  source = "../../../terraform-modules/master-node"

  clusterConfiguration         = var.clusterConfiguration
  providerClusterConfiguration = var.providerClusterConfiguration
  clusterUUID                  = var.clusterUUID
  nodeIndex                    = var.nodeIndex
  cloudConfig                  = var.cloudConfig
  resourceManagementTimeout    = var.resourceManagementTimeout
}

locals {
  useNSXV        = var.providerClusterConfiguration.edgeGateway.type == "NSX-V"
  edgeGatewayId  = local.useNSXV ? data.vcd_edgegateway.gateway[0].id : data.vcd_nsxt_edgegateway.gateway[0].id
}

data "vcd_nsxt_edgegateway" "gateway" {
  count = local.useNSXV ? 0 : 1
  org   = var.providerClusterConfiguration.organization
  name  = var.providerClusterConfiguration.edgeGateway.name
}

data "vcd_edgegateway" "gateway" {
  count = local.useNSXV ? 1 : 0
  org   = var.providerClusterConfiguration.organization
  name  = var.providerClusterConfiguration.edgeGateway.name
}

module "dnat" {
  source = "../../../terraform-modules/dnat"

  providerClusterConfiguration = var.providerClusterConfiguration
  edgeGatewayId                = local.edgeGatewayId
  useNSXV                      = local.useNSXV
  nodeIndex                    = var.nodeIndex
  master_node_ip               = module.master_node.master_ip_address_for_ssh
}
