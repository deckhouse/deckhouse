variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "clusterUUID" {
  type = string
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  vpc_network_cidr = lookup(var.providerClusterConfiguration, "vpcNetworkCIDR", "")
  existing_vpc_id = lookup(var.providerClusterConfiguration, "existingVPCID", "")
  tags = lookup(var.providerClusterConfiguration, "tags", {})
}
