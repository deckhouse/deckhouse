# Copyright 2021 Flant CJSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "nodeIndex" {
  type = string
  default = ""
}

variable "cloudConfig" {
  type = string
  default = ""
}

variable "clusterUUID" {
  type = string
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  pod_subnet_cidr = var.clusterConfiguration.podSubnetCIDR
  external_network_name = var.providerClusterConfiguration.simple.externalNetworkName
  external_network_dhcp = lookup(var.providerClusterConfiguration.simple, "externalNetworkDHCP", true)
  pod_network_mode = lookup(var.providerClusterConfiguration.simple, "podNetworkMode", "VXLAN")
  image_name = var.providerClusterConfiguration.masterNodeGroup.instanceClass.imageName
  tags = lookup(var.providerClusterConfiguration, "tags", {})
}
