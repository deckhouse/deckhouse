# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
  validation {
    condition     = cidrsubnet(var.providerClusterConfiguration.vpcPeering.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.vpcPeering.internalNetworkCIDR
    error_message = "Invalid internalNetworkCIDR in HuaweiCloudClusterConfiguration."
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

variable "mainNetwork" {
  type        = string
  default     = null
}

variable "additionalNetworks" {
  type        = list(string)
  default     = []
}

locals {
  prefix                = var.clusterConfiguration.cloud.prefix
  pod_subnet_cidr       = var.clusterConfiguration.podSubnetCIDR
  internal_network_cidr = var.providerClusterConfiguration.vpcPeering.internalNetworkCIDR
  network_security      = lookup(var.providerClusterConfiguration.vpcPeering, "internalNetworkSecurity", true)
  image_name            = var.providerClusterConfiguration.masterNodeGroup.instanceClass.imageName
  tags                  = lookup(var.providerClusterConfiguration, "tags", {})
  ssh_allow_list        = lookup(var.providerClusterConfiguration, "sshAllowList", ["0.0.0.0/0"])
  server_group          = lookup(var.providerClusterConfiguration.masterNodeGroup, "serverGroup", {})
  server_group_policy   = lookup(local.server_group, "policy", "")
  enterprise_project_id = lookup(var.providerClusterConfiguration.provider, "enterpriseProjectID", "")
}
