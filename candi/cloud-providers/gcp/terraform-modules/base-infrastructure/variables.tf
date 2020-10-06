variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
  validation {
    condition     = contains(keys(var.providerClusterConfiguration), "subnetworkCIDR") ? cidrsubnet(var.providerClusterConfiguration.subnetworkCIDR, 0, 0) == var.providerClusterConfiguration.subnetworkCIDR : true
    error_message = "Invalid subnetworkCIDR in GCPClusterConfiguration."
  }
}

variable "nodeIndex" {
  type    = string
  default = ""
}

variable "cloudConfig" {
  type    = string
  default = ""
}

locals {
  prefix              = var.clusterConfiguration.cloud.prefix
  pod_subnet_cidr     = lookup(var.clusterConfiguration, "podSubnetCIDR", "10.100.0.0/16")
  subnetwork_cidr     = lookup(var.providerClusterConfiguration, "subnetworkCIDR", "10.172.0.0/16")
  cloud_nat_addresses = var.providerClusterConfiguration.layout == "Standard" && lookup(var.providerClusterConfiguration, "standard", false) ? lookup(var.providerClusterConfiguration.standard, "cloudNATAddresses", []) : []
  peered_vpcs_names   = lookup(var.providerClusterConfiguration, "peeredVPCs", [])
}
