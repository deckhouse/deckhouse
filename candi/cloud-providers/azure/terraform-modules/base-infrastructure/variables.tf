variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
  validation {
    condition     = contains(keys(var.providerClusterConfiguration), "vNetCIDR") ? cidrsubnet(var.providerClusterConfiguration.vNetCIDR, 0, 0) == var.providerClusterConfiguration.vNetCIDR : true
    error_message = "Invalid vNetCIDR in AzureClusterConfiguration."
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

variable "clusterUUID" {
  type = string
}

locals {
  prefix                      = var.clusterConfiguration.cloud.prefix
  location                    = var.providerClusterConfiguration.provider.location
  vnet_cidr                   = var.providerClusterConfiguration.vNetCIDR
  subnet_cidr                 = var.providerClusterConfiguration.subnetCIDR
  zones                       = lookup(var.providerClusterConfiguration.masterNodeGroup, "zones", ["1", "2", "3"])
  peered_vnets                = { for vpc in lookup(var.providerClusterConfiguration, "peeredVNets", []) : vpc.vnetName => vpc }
  enable_nat_gateway          = lookup(var.providerClusterConfiguration, "enableNatGateway", false)
  additional_tags             = lookup(var.providerClusterConfiguration, "tags", {})
  nat_gateway_public_ip_count = contains(keys(var.providerClusterConfiguration), "standard") ? lookup(var.providerClusterConfiguration.standard, "natGatewayPublicIpCount", 0) : 0
}
