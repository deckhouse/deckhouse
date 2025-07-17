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
  use_nsxv              = var.providerClusterConfiguration.edgeGateway.type == "NSX-V"
  external_network_name = contains(keys(var.providerClusterConfiguration.edgeGateway), "NSX-V") ? var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkName : null
  external_network_type = contains(keys(var.providerClusterConfiguration.edgeGateway), "NSX-V") ? var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkType : null
}

module "dnat" {
  source = "../../../terraform-modules/dnat"

  organization          = var.providerClusterConfiguration.organization
  edge_gateway_name     = var.providerClusterConfiguration.edgeGateway.name
  edge_gateway_type     = var.providerClusterConfiguration.edgeGateway.type
  internal_network_name = var.providerClusterConfiguration.mainNetwork
  internal_address      = module.master_node.master_ip_address_for_ssh
  external_address      = var.providerClusterConfiguration.edgeGateway.externalIP
  external_port         = var.providerClusterConfiguration.edgeGateway.externalPort
  external_network_name = local.external_network_name
  external_network_type = local.external_network_type
  node_index            = var.nodeIndex
}
