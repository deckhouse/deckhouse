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

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  pod_subnet_cidr = var.clusterConfiguration.podSubnetCIDR
  internal_network_cidr = var.providerClusterConfiguration.standard.internalNetworkCIDR
  external_network_name = var.providerClusterConfiguration.standard.externalNetworkName
  network_security = lookup(var.providerClusterConfiguration.standard, "internalNetworkSecurity", true)
}
