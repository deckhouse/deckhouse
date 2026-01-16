# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any

  validation {
    condition     = contains(keys(var.providerClusterConfiguration), "internalNetworkCIDR") ? cidrsubnet(var.providerClusterConfiguration.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.internalNetworkCIDR : true
    error_message = "Invalid internalNetworkCIDR in VCDClusterConfiguration."
  }

  validation {
    condition     = contains(keys(var.providerClusterConfiguration.masterNodeGroup.instanceClass), "mainNetworkIPAddresses") ? length(var.providerClusterConfiguration.masterNodeGroup.instanceClass.mainNetworkIPAddresses) == length([for s in var.providerClusterConfiguration.masterNodeGroup.instanceClass.mainNetworkIPAddresses : s if cidrsubnet(format("%s/%s", s, split("/", var.providerClusterConfiguration.internalNetworkCIDR)[1]), 0, 0) == var.providerClusterConfiguration.internalNetworkCIDR]) : true
    error_message = "Address in mainNetworkIPAddresses not in internalNetworkCIDR."
  }

}

variable "nodeIndex" {
  type    = number
  default = 0
}

variable "cloudConfig" {
  type    = string
  default = ""
}

variable "clusterUUID" {
  type = string
}

variable "resourceManagementTimeout" {
  type    = string
  default = "10m"
}

variable "registryDataDeviceEnable" {
  type    = bool
  default = false
}

locals {
  prefix                = var.clusterConfiguration.cloud.prefix
  vapp_name             = var.providerClusterConfiguration.virtualApplicationName
  master_instance_class = var.providerClusterConfiguration.masterNodeGroup.instanceClass
  main_ip_addresses     = lookup(local.master_instance_class, "mainNetworkIPAddresses", [])
  main_network_name     = var.providerClusterConfiguration.mainNetwork
  metadata = merge(
    (contains(keys(var.providerClusterConfiguration), "metadata") ? var.providerClusterConfiguration.metadata : {}),
    (contains(keys(local.master_instance_class), "additionalMetadata") ? local.master_instance_class.additionalMetadata : {}),
  )
}
