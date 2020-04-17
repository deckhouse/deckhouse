variable "clusterConfig" {
  type = any
}

variable "clusterProviderConfig" {
  type = any
}

locals {
  prefix = var.clusterConfig.spec.cloud.prefix
  internal_network_cidr = var.clusterProviderConfig.spec.standard.internalNetworkCIDR
  external_network_name = var.clusterProviderConfig.spec.standard.externalNetworkName
  network_security = var.clusterProviderConfig.spec.standard.internalNetworkSecurity
}
