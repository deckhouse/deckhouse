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
  internal_network_cidr = var.providerClusterConfig.standardWithNoRouter.internalNetworkCIDR
  external_network_name = var.providerClusterConfig.standardWithNoRouter.externalNetworkName
  external_network_dhcp = lookup(var.providerClusterConfig.standardWithNoRouter, "externalNetworkDHCP", true)
  network_security = var.providerClusterConfig.standardWithNoRouter.internalNetworkSecurity
}
