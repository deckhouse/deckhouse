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
