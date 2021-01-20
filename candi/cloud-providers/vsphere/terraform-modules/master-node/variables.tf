variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any

  validation {
    condition = cidrsubnet(var.providerClusterConfiguration.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.internalNetworkCIDR
    error_message = "Invalid internalNetworkCIDR in VsphereClusterConfiguration."
  }
}

variable "nodeIndex" {
  type = number
  default = 0
}

variable "cloudConfig" {
  type = string
  default = ""
}

variable "clusterUUID" {
  type = string
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix

  mng = var.providerClusterConfiguration.masterNodeGroup
  master_instance_class = var.providerClusterConfiguration.masterNodeGroup.instanceClass

  config_zones = var.providerClusterConfiguration.zones
  ng_zones = lookup(local.mng, "zones", [])
  effective_zones = length(local.ng_zones) > 0 ? local.ng_zones : local.config_zones
  zone = element(local.effective_zones, var.nodeIndex)

  base_resource_pool = trim(lookup(var.providerClusterConfiguration, "baseResourcePool", ""), "/")
  default_resource_pool = join("/", local.base_resource_pool != "" ? [local.base_resource_pool, local.prefix] : [local.prefix])

  resource_pool = lookup(local.master_instance_class, "resourcePool", local.default_resource_pool)
  additionalNetworks = lookup(local.master_instance_class, "additionalNetworks", [])

  runtime_options = lookup(local.master_instance_class, "runtimeOptions", {})
  calculated_memory_reservation = lookup(local.runtime_options, "memoryReservation", 80)
}
