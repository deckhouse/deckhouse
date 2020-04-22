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
  internal_network_cidr = var.providerClusterConfig.standard.internalNetworkCIDR
  external_network_name = var.providerClusterConfig.standard.externalNetworkName
  network_security = var.providerClusterConfig.standard.internalNetworkSecurity
}
