# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
  validation {
    condition     = cidrsubnet(var.providerClusterConfiguration.standard.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.standard.internalNetworkCIDR
    error_message = "Invalid internalNetworkCIDR in OpenStackClusterConfiguration."
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

variable "resourceManagementTimeout" {
  type = string
  default = "10m"
}

locals {
  prefix                = var.clusterConfiguration.cloud.prefix
  standard              = lookup(var.providerClusterConfiguration, "standard", {})
  pod_subnet_cidr       = var.clusterConfiguration.podSubnetCIDR
  internal_network_cidr = var.providerClusterConfiguration.standard.internalNetworkCIDR
  external_network_name = var.providerClusterConfiguration.standard.externalNetworkName
  network_security      = lookup(var.providerClusterConfiguration.standard, "internalNetworkSecurity", true)
  image_name            = var.providerClusterConfiguration.masterNodeGroup.instanceClass.imageName
  tags                  = lookup(var.providerClusterConfiguration, "tags", {})
  ssh_allow_list        = lookup(var.providerClusterConfiguration, "sshAllowList", ["0.0.0.0/0"])
  server_group          = lookup(var.providerClusterConfiguration.masterNodeGroup, "serverGroup", {})
  server_group_policy   = lookup(local.server_group, "policy", "")
}
