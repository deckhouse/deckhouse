variable "clusterConfig" {
  type = any
}

variable "providerClusterConfig" {
  type = any
}

variable "initConfig" {
  type = any
}

variable "providerInitConfig" {
  type = any
}

locals {
  prefix = var.clusterConfig.cloud.prefix
  pod_subnet_cidr = var.clusterConfig.podSubnetCIDR
  internal_subnet_name = var.providerClusterConfig.simpleWithInternalNetwork.internalSubnetName
  external_network_name = lookup(var.providerClusterConfig.simpleWithInternalNetwork, "externalNetworkName", "")
  pod_network_mode = lookup(var.providerClusterConfig.simpleWithInternalNetwork, "podNetworkMode", "DirectRoutingWithPortSecurityEnabled")
}
