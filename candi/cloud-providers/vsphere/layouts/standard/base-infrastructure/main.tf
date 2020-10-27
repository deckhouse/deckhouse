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
