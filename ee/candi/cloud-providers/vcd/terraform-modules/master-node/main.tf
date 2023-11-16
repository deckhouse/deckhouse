# Copyright 2023 Flant JSC
# Licensed underthe Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE


locals {
  catalog  = split("/", local.master_instance_class.template)[0]
  template = split("/", local.master_instance_class.template)[1]
  ip_address  = length(local.main_ip_addresses) > 0 ? element(local.main_ip_addresses, var.nodeIndex) : null
}

data "vcd_catalog" "catalog" {
  name = local.catalog
}

data "vcd_catalog_vapp_template" "template" {
  catalog_id = data.vcd_catalog.catalog.id
  name       = local.template
}

data "vcd_storage_profile" "sp" {
  name = local.master_instance_class.storageProfile
}
/*
resource "vcd_independent_disk" "kubernetes_data" {
  name            = "kubernetes-data"
  size_in_mb      = local.master_instance_class.etcdDiskSizeGb * 1024
  bus_type        = "SCSI"
  bus_sub_type    = "VirtualSCSI"
  storage_profile = local.master_instance_class.storageProfile == null ? "" : local.master_instance_class.storageProfile
  iops            = data.vcd_storage_profile.sp.iops_settings[0].disk_iops_per_gb_max * local.master_instance_class.etcdDiskSizeGb
}
*/
resource "vcd_vm" "master" {
  name             = join("-", [local.prefix, "master", var.nodeIndex])
  computer_name    = join("-", [local.prefix, "master", var.nodeIndex])
  vapp_template_id = data.vcd_catalog_vapp_template.template.id

  cpus = local.master_instance_class.numCPUs
  memory   = local.master_instance_class.memory
  memory_hot_add_enabled = true

  network {
    name               = "internal"
    type               = "org"
    ip_allocation_mode = local.ip_address == null ? "DHCP" : "MANUAL"
    is_primary         = true
    ip                 = local.ip_address
  }

  override_template_disk {
    bus_type        = "paravirtual"
    size_in_mb      = local.master_instance_class.rootDiskSizeGb * 1024
    bus_number      = 0
    unit_number     = 0
    storage_profile = local.master_instance_class.storageProfile == null ? "" : local.master_instance_class.storageProfile
    iops            = data.vcd_storage_profile.sp.iops_settings[0].disk_iops_per_gb_max * local.master_instance_class.rootDiskSizeGb
  }

  /*
  disk {
    name = vcd_independent_disk.kubernetes_data.name
    bus_number = 1
    unit_number = 0
  }
*/
  guest_properties = {
    "instance-id"         = join("-", [local.prefix, "master", var.nodeIndex])
    "local-hostname"      = join("-", [local.prefix, "master", var.nodeIndex])
    "public-keys"         = var.providerClusterConfiguration.sshPublicKey
  }
}
