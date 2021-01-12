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

resource "vsphere_folder" "main" {
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
}

data "vsphere_resource_pool" "parent_resource_pool" {
  for_each = toset([for s in data.vsphere_dynamic.cluster_id: s.inventory_path])
  name          = join("/", local.base_resource_pool != "" ? [each.key, "Resources", local.base_resource_pool] : [each.key, "Resources"])
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

resource "vsphere_resource_pool" "resource_pool" {
  for_each = toset([for s in data.vsphere_resource_pool.parent_resource_pool: s.id])
  name          = local.prefix
  parent_resource_pool_id = each.key
}
