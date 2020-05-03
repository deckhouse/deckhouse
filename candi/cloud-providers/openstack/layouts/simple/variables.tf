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
  external_network_name = var.providerClusterConfig.simple.externalNetworkName
  external_network_dhcp = lookup(var.providerClusterConfig.simple, "externalNetworkDHCP", true)
  pod_network_mode = lookup(var.providerClusterConfig.simple, "podNetworkMode", "VXLAN")
}
