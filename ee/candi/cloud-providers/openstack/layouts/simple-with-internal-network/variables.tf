# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "nodeIndex" {
  type    = string
  default = ""
}

variable "cloudConfig" {
  type    = string
  default = ""
}

variable "clusterUUID" {
  type = string
}

locals {
  prefix                = var.clusterConfiguration.cloud.prefix
  pod_subnet_cidr       = var.clusterConfiguration.podSubnetCIDR
  internal_subnet_name  = var.providerClusterConfiguration.simpleWithInternalNetwork.internalSubnetName
  external_network_name = lookup(var.providerClusterConfiguration.simpleWithInternalNetwork, "externalNetworkName", "")
  pod_network_mode      = lookup(var.providerClusterConfiguration.simpleWithInternalNetwork, "podNetworkMode", "DirectRoutingWithPortSecurityEnabled")
  image_name            = var.providerClusterConfiguration.masterNodeGroup.instanceClass.imageName
  tags                  = lookup(var.providerClusterConfiguration, "tags", {})
  bind_volumes_to_zone  = lookup(var.providerClusterConfiguration, "bindVolumesToZone", false)
}
