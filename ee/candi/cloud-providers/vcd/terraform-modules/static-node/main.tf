# Copyright 2023 Flant JSC
# Licensed underthe Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  catalog  = split("/", local.instance_class.template)[0]
  template = split("/", local.instance_class.template)[1]
  ip_address  = length(local.main_ip_addresses) > 0 ? element(local.main_ip_addresses, var.nodeIndex) : null
  placement_policy = lookup(local.instance_class, "placementPolicy", "")
}

data "vcd_catalog" "catalog" {
  name = local.catalog
}

data "vcd_catalog_vapp_template" "template" {
  catalog_id = data.vcd_catalog.catalog.id
  name       = local.template
}

data "vcd_storage_profile" "sp" {
  name = local.instance_class.storageProfile
}

data "vcd_vm_sizing_policy" "vmsp" {
  name = local.instance_class.sizingPolicy
}

data "vcd_org_vdc" "vdc" {
  name = var.providerClusterConfiguration.virtualDataCenter
  org = var.providerClusterConfiguration.organization
}

data "vcd_vm_placement_policy" "vmpp" {
  count = local.placement_policy == "" ? 0 : 1
  name = local.placement_policy
  vdc_id = data.vcd_org_vdc.vdc.id
}

resource "vcd_vm" "node" {
  name             = join("-", [local.prefix, local.node_group_name, var.nodeIndex])
  computer_name    = join("-", [local.prefix, local.node_group_name, var.nodeIndex])
  vapp_template_id = data.vcd_catalog_vapp_template.template.id


  sizing_policy_id = data.vcd_vm_sizing_policy.vmsp.id
  placement_policy_id = local.placement_policy == "" ? "" : data.vcd_vm_placement_policy.vmpp[0].id

  network {
    name               = "internal"
    type               = "org"
    ip_allocation_mode = local.ip_address == null ? "DHCP" : "MANUAL"
    is_primary         = true
    ip                 = local.ip_address
  }

  override_template_disk {
    bus_type        = "paravirtual"
    size_in_mb      = local.instance_class.rootDiskSizeGb * 1024
    bus_number      = 0
    unit_number     = 0
    storage_profile = data.vcd_storage_profile.sp.name
    iops            = data.vcd_storage_profile.sp.iops_settings[0].disk_iops_per_gb_max * local.instance_class.rootDiskSizeGb
  }

  guest_properties = {
    "instance-id"         = join("-", [local.prefix, local.node_group_name, var.nodeIndex])
    "local-hostname"      = join("-", [local.prefix, local.node_group_name, var.nodeIndex])
    "public-keys"         = var.providerClusterConfiguration.sshPublicKey
  }
}
