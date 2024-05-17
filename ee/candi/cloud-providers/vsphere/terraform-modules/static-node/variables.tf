# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any

  validation {
    condition     = cidrsubnet(var.providerClusterConfiguration.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.internalNetworkCIDR
    error_message = "Invalid internalNetworkCIDR in VsphereClusterConfiguration."
  }

  validation {
    condition     = length(flatten([for ng in var.providerClusterConfiguration.nodeGroups : ng.instanceClass.mainNetworkIPAddresses if contains(keys(ng.instanceClass), "mainNetworkIPAddresses")])) == length(flatten([for ng in var.providerClusterConfiguration.nodeGroups : [for a in ng.instanceClass.mainNetworkIPAddresses : a if a.address != cidrsubnet(a.address, 0, 0)] if contains(keys(ng.instanceClass), "mainNetworkIPAddresses")]))
    error_message = "Invalid address in mainNetworkIPAddresses."
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

variable "wait_for_guest_net_routable" {
  type = bool
  default = false
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix

  ng             = [for i in var.providerClusterConfiguration.nodeGroups : i if i.name == var.nodeGroupName][0]
  instance_class = local.ng["instanceClass"]

  node_group_name = local.ng.name

  actual_zones    = var.providerClusterConfiguration.zones
  zones           = lookup(local.ng, "zones", null) != null ? tolist(setintersection(local.actual_zones, local.ng["zones"])) : local.actual_zones
  zone            = element(local.zones, var.nodeIndex)

  use_nested_resource_pool = lookup(var.providerClusterConfiguration, "useNestedResourcePool", true)
  base_resource_pool    = trim(lookup(var.providerClusterConfiguration, "baseResourcePool", ""), "/")
  default_resource_pool = local.use_nested_resource_pool == true ? join("/", local.base_resource_pool != "" ? [local.base_resource_pool, local.prefix] : [local.prefix]) : ""

  resource_pool = lookup(local.instance_class, "resourcePool", local.default_resource_pool)

  additionalNetworks = lookup(local.instance_class, "additionalNetworks", [])
  main_ip_addresses  = lookup(local.instance_class, "mainNetworkIPAddresses", [])

  runtime_options               = lookup(local.instance_class, "runtimeOptions", {})
  calculated_memory_reservation = lookup(local.runtime_options, "memoryReservation", 80)
}
