variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any

  validation {
    condition = contains(keys(var.providerClusterConfiguration), "vpcNetworkCIDR") ? cidrsubnet(var.providerClusterConfiguration.vpcNetworkCIDR,0,0) == var.providerClusterConfiguration.vpcNetworkCIDR : true
    error_message = "Invalid vpcNetworkCIDR in AWSClusterConfiguration."
  }

  validation {
    condition = contains(keys(var.providerClusterConfiguration), "nodeNetworkCIDR") ? cidrsubnet(var.providerClusterConfiguration.nodeNetworkCIDR,0,0) == var.providerClusterConfiguration.nodeNetworkCIDR : true
    error_message = "Invalid nodeNetworkCIDR in AWSClusterConfiguration."
  }
}

variable "clusterUUID" {
  type = string
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  vpc_network_cidr = lookup(var.providerClusterConfiguration, "vpcNetworkCIDR", "")
  existing_vpc_id = lookup(var.providerClusterConfiguration, "existingVPCID", "")
}
