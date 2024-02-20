# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any

  validation {
    condition     = cidrsubnet(var.providerClusterConfiguration.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.internalNetworkCIDR
    error_message = "Invalid internalNetworkCIDR in VCDClusterConfiguration."
  }

  validation {
    condition     = length(flatten([for ng in var.providerClusterConfiguration.nodeGroups : ng.instanceClass.mainNetworkIPAddresses if contains(keys(ng.instanceClass), "mainNetworkIPAddresses")])) == length(flatten([for ng in var.providerClusterConfiguration.nodeGroups : [for s in ng.instanceClass.mainNetworkIPAddresses : s if cidrsubnet(format("%s/%s", s, split("/", var.providerClusterConfiguration.internalNetworkCIDR)[1]), 0, 0) == var.providerClusterConfiguration.internalNetworkCIDR] if contains(keys(ng.instanceClass), "mainNetworkIPAddresses")]))
    error_message = "Address in mainNetworkIPAddresses not in internalNetworkCIDR."
  }
}

variable "nodeIndex" {
  type    = number
  default = 0
}

variable "cloudConfig" {
  type = string
  default = ""
}

variable "clusterUUID" {
  type = string
}

variable "nodeGroupName" {
  type = string
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  vapp_name = var.providerClusterConfiguration.virtualApplicationName
  master_instance_class = var.providerClusterConfiguration.masterNodeGroup.instanceClass
  ng             = [for i in var.providerClusterConfiguration.nodeGroups : i if i.name == var.nodeGroupName][0]
  instance_class = local.ng["instanceClass"]
  node_group_name = local.ng.name
  main_ip_addresses  = lookup(local.instance_class, "mainNetworkIPAddresses", [])
  main_network_name = var.providerClusterConfiguration.mainNetwork
}
