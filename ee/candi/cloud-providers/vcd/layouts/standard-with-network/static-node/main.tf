# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "vcd_vapp_network_routed" "network" {
  vapp_name = var.providerClusterConfiguration.virtualApplicationName
  name      = var.providerClusterConfiguration.prefix
}

module "static_node" {
  source = "../../../terraform-modules/static-node"

  clusterConfiguration         = var.clusterConfiguration
  providerClusterConfiguration = var.providerClusterConfiguration
  nodeIndex                    = var.nodeIndex
  clusterUUID                  = var.clusterUUID
  nodeGroupName                = var.nodeGroupName

  virtualApplicationName       = var.providerClusterConfiguration.virtualApplicationName
  networkName                  = data.vcd_vapp_network_routed.network.name
}
