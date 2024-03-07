# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "vsphere_tag_category" "region" {
  name = var.providerClusterConfiguration.regionTagCategory
}

data "vsphere_tag" "region_tag" {
  name        = var.providerClusterConfiguration.region
  category_id = data.vsphere_tag_category.region.id
}

data "vsphere_dynamic" "datacenter_id" {
  filter     = [data.vsphere_tag.region_tag.id]
  type       = "Datacenter"
}

data "vsphere_folder" "main" {
  count         = var.providerClusterConfiguration.vmFolderExists ? 1 : 0
  path = format("/%s/vm/%s", var.providerClusterConfiguration.region, var.providerClusterConfiguration.vmFolderPath)
}

resource "vsphere_folder" "main" {
  count         = var.providerClusterConfiguration.vmFolderExists ? 0 : 1
  path          = var.providerClusterConfiguration.vmFolderPath
  type          = "vm"
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

data "vsphere_tag_category" "zone" {
  name = var.providerClusterConfiguration.zoneTagCategory
}

data "vsphere_tag" "zone_tag" {
  for_each = toset(var.providerClusterConfiguration.zones)
  name        = each.key
  category_id = data.vsphere_tag_category.zone.id
}

data "vsphere_dynamic" "cluster_id" {
  for_each = toset([for s in data.vsphere_tag.zone_tag: s.id])
  filter     = [each.key]
  type       = "ClusterComputeResource"
  resolve_inventory_path = true
}

locals {
  base_resource_pool = trim(lookup(var.providerClusterConfiguration, "baseResourcePool", ""), "/")
  use_nested_resource_pool = lookup(var.providerClusterConfiguration, "useNestedResourcePool", true)
}

data "vsphere_resource_pool" "parent_resource_pool" {
  for_each = toset([for s in data.vsphere_dynamic.cluster_id: s.inventory_path])
  name          = join("/", local.base_resource_pool != "" ? [each.key, "Resources", local.base_resource_pool] : [each.key, "Resources"])
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

resource "vsphere_resource_pool" "resource_pool" {
  for_each = toset(local.use_nested_resource_pool == true ? [for s in data.vsphere_resource_pool.parent_resource_pool: s.id ] : [])
  name          = local.prefix
  parent_resource_pool_id = each.key

  # for now we assume that these options would be tuned manually by vSphere cluster admin
  lifecycle {
    ignore_changes = [
      cpu_share_level,
      cpu_shares,
      cpu_reservation,
      cpu_expandable,
      cpu_limit,
      memory_share_level,
      memory_shares,
      memory_reservation,
      memory_expandable,
      memory_limit
    ]
  }
}
