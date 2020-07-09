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
  internal_network_cidr = var.providerClusterConfiguration.standardWithNoRouter.internalNetworkCIDR
  external_network_name = var.providerClusterConfiguration.standardWithNoRouter.externalNetworkName
  external_network_dhcp = lookup(var.providerClusterConfiguration.standardWithNoRouter, "externalNetworkDHCP", true)
  network_security = var.providerClusterConfiguration.standardWithNoRouter.internalNetworkSecurity
}
