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
  internal_network_cidr = var.providerClusterConfig.standard.internalNetworkCIDR
  external_network_name = var.providerClusterConfig.standard.externalNetworkName
  network_security = lookup(var.providerClusterConfig.standard, "internalNetworkSecurity", true)
}
