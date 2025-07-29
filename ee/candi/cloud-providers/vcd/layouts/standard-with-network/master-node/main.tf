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
