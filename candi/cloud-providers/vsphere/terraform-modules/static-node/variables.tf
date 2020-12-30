variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any

  validation {
    condition     = cidrsubnet(var.providerClusterConfiguration.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.internalNetworkCIDR
    error_message = "Invalid internalNetworkCIDR in VsphereClusterConfiguration."
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

variable "nodeGroupName" {
  type = string
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix

  ng             = [for i in var.providerClusterConfiguration.nodeGroups : i if i.name == var.nodeGroupName][0]
  instance_class = local.ng["instanceClass"]

  node_group_name = local.ng.name

  config_zones    = var.providerClusterConfiguration.zones
  ng_zones        = lookup(local.ng, "zones", [])
  effective_zones = length(local.ng_zones) > 0 ? local.ng_zones : local.config_zones
  zone            = element(local.effective_zones, var.nodeIndex)

  resourcePool       = lookup(local.instance_class, "resourcePool", "")
  additionalNetworks = lookup(local.instance_class, "additionalNetworks", [])

  runtime_options               = lookup(local.instance_class, "runtimeOptions", {})
  calculated_memory_reservation = lookup(local.runtime_options, "memoryReservation", 80)
}
