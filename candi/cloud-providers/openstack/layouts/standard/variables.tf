variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
  validation {
    condition = cidrsubnet(var.providerClusterConfiguration.standard.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.standard.internalNetworkCIDR
    error_message = "Invalid internalNetworkCIDR in OpenStackClusterConfiguration."
  }
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
  image_name = var.providerClusterConfiguration.masterNodeGroup.instanceClass.imageName
  tags = lookup(var.providerClusterConfiguration, "tags", {})
}
