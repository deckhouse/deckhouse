# Copyright 2023 Flant JSC
# Licensed underthe Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE


locals {
  template_parts   = split("/", local.master_instance_class.template)
  org              = length(local.template_parts) == 3 ? local.template_parts[0] : null
  catalog          = length(local.template_parts) == 3 ? local.template_parts[1] : local.template_parts[0]
  template         = length(local.template_parts) == 3 ? local.template_parts[2] : local.template_parts[1]
  ip_address       = length(local.main_ip_addresses) > 0 ? element(local.main_ip_addresses, var.nodeIndex) : null
  placement_policy = lookup(local.master_instance_class, "placementPolicy", "")
}

// hack to recreate VM when changing kubernetes_data.id, must be replaced with replace_triggered_by after tf upgrade
locals {
  disk_hash    = md5(vcd_independent_disk.kubernetes_data.id)
  disk_offset  = parseint(local.disk_hash, 16) % 21 + 1
}

data "vcd_catalog" "catalog" {
  org  = local.org
  name = local.catalog
}

data "vcd_catalog_vapp_template" "template" {
  catalog_id = data.vcd_catalog.catalog.id
  name       = local.template
}

data "vcd_storage_profile" "sp" {
  name = local.master_instance_class.storageProfile
}

data "vcd_vm_sizing_policy" "vmsp" {
  name = local.master_instance_class.sizingPolicy
}

data "vcd_org_vdc" "vdc" {
  count = local.placement_policy == "" ? 0 : 1
  name = var.providerClusterConfiguration.virtualDataCenter
  org = var.providerClusterConfiguration.organization
}

data "vcd_vm_placement_policy" "vmpp" {
  count = local.placement_policy == "" ? 0 : 1
  name = local.placement_policy
  vdc_id = data.vcd_org_vdc.vdc[0].id
}

resource "vcd_independent_disk" "kubernetes_data" {
  name             = "${local.prefix}-master-${var.nodeIndex}-etcd-disk"
  size_in_mb       = local.master_instance_class.etcdDiskSizeGb * 1024
  storage_profile  = data.vcd_storage_profile.sp.name
  bus_type        = "SCSI"
  bus_sub_type    = "VirtualSCSI"
}

resource "vcd_vapp_vm" "master" {
  vapp_name        = local.vapp_name
  name             = join("-", [local.prefix, "master", var.nodeIndex])
  computer_name = join("-", [local.prefix, "master", var.nodeIndex])
  vapp_template_id = data.vcd_catalog_vapp_template.template.id
  sizing_policy_id = data.vcd_vm_sizing_policy.vmsp.id
  placement_policy_id = local.placement_policy == "" ? "" : data.vcd_vm_placement_policy.vmpp[0].id

  network {
    name               = local.main_network_name
    type               = "org"
    ip_allocation_mode = local.ip_address == null ? "DHCP" : "MANUAL"
    is_primary         = true
    ip                 = local.ip_address
  }
  network_dhcp_wait_seconds = 120

  override_template_disk {
    bus_type        = "paravirtual"
    // disk_offset is just a hack to recreate VM when changing kubernetes_data.id, must be replaced with replace_triggered_by after tf upgrade
    // this will add 0-20 mbytes to root disk size
    size_in_mb      = (local.master_instance_class.rootDiskSizeGb * 1024) + local.disk_offset
    bus_number      = 0
    unit_number     = 0
    storage_profile = data.vcd_storage_profile.sp.name
    iops            = data.vcd_storage_profile.sp.iops_settings[0].disk_iops_per_gb_max > 0 ? data.vcd_storage_profile.sp.iops_settings[0].disk_iops_per_gb_max * local.master_instance_class.rootDiskSizeGb : ( data.vcd_storage_profile.sp.iops_settings[0].default_disk_iops > 0 ?  data.vcd_storage_profile.sp.iops_settings[0].default_disk_iops : 0)
  }

  disk {
    name        = vcd_independent_disk.kubernetes_data.name
    bus_number  = 0
    unit_number = 1
  }

  customization {
    force = false
    enabled = true
  }

  lifecycle {
    ignore_changes = [
      guest_properties,
      metadata
    ]
  }

  depends_on = [
    vcd_independent_disk.kubernetes_data
  ]

  guest_properties = merge({
    "instance-id"         = join("-", [local.prefix, "master", var.nodeIndex])
    "local-hostname"      = join("-", [local.prefix, "master", var.nodeIndex])
    "public-keys"         = var.providerClusterConfiguration.sshPublicKey
    "disk.EnableUUID"     = "1"
  }, length(var.cloudConfig) > 0 ? {"user-data" = var.cloudConfig} : {} )
}
