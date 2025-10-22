# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "vcd_catalog" "catalog" {
  org  = local.org
  name = local.catalog
}

data "vcd_catalog_vapp_template" "template" {
  catalog_id = data.vcd_catalog.catalog.id
  name       = local.template
}

data "vcd_storage_profile" "sp" {
  name = var.storage_profile
}

data "vcd_vm_sizing_policy" "vmsp" {
  name = var.sizing_policy
}

data "vcd_org_vdc" "vdc" {
  count = var.placement_policy == "" ? 0 : 1
  name  = var.vdc_name
  org   = var.organization
}

data "vcd_vm_placement_policy" "vmpp" {
  count  = var.placement_policy == "" ? 0 : 1
  name   = var.placement_policy
  vdc_id = data.vcd_org_vdc.vdc[0].id
}

resource "vcd_vapp_vm" "bastion" {
  vapp_name        = var.vapp_name
  name             = local.name
  computer_name    = local.name
  vapp_template_id = data.vcd_catalog_vapp_template.template.id


  sizing_policy_id    = data.vcd_vm_sizing_policy.vmsp.id
  placement_policy_id = var.placement_policy == "" ? "" : data.vcd_vm_placement_policy.vmpp[0].id

  network {
    name               = var.network_name
    type               = "org"
    ip_allocation_mode = var.ip_address == null ? "DHCP" : "MANUAL"
    is_primary         = true
    ip                 = var.ip_address
  }
  network_dhcp_wait_seconds = 120

  override_template_disk {
    bus_type        = "paravirtual"
    size_in_mb      = var.root_disk_size_gb * 1024
    bus_number      = 0
    unit_number     = 0
    storage_profile = data.vcd_storage_profile.sp.name
    iops            = data.vcd_storage_profile.sp.iops_settings[0].disk_iops_per_gb_max > 0 ? data.vcd_storage_profile.sp.iops_settings[0].disk_iops_per_gb_max * var.root_disk_size_gb : (data.vcd_storage_profile.sp.iops_settings[0].default_disk_iops > 0 ? data.vcd_storage_profile.sp.iops_settings[0].default_disk_iops : 0)
  }

  customization {
    force   = false
    enabled = true
  }

  lifecycle {
    ignore_changes = [
      guest_properties,
      disk,
      metadata
    ]
  }

  guest_properties = {
    "instance-id"     = local.name
    "local-hostname"  = local.name
    "public-keys"     = var.ssh_public_key
    "disk.EnableUUID" = "1"
  }

  dynamic "metadata_entry" {
    for_each = var.metadata

    content {
      type        = "MetadataStringValue"
      is_system   = false
      user_access = "READWRITE"
      key         = metadata_entry.key
      value       = metadata_entry.value
    }
  }
}
