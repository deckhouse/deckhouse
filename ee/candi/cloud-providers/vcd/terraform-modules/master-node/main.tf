# Copyright 2023 Flant JSC
# Licensed underthe Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE


locals {
  catalog                 = split("/", local.master_instance_class.template)[0]
  template                = split("/", local.master_instance_class.template)[1]
  external_addresses      = length(local.main_ip_addresses) > 0 ? element(local.main_ip_addresses, var.nodeIndex) : tomap({})
  external_ip             = lookup(local.external_addresses, "address", null)
  external_gateway        = lookup(local.external_addresses, "gateway", null)
  external_nameservers    = local.external_addresses == null ? null : lookup(local.external_addresses, "nameservers", null)
  external_dns_addresses  = local.external_nameservers == null ? null : lookup(local.external_nameservers, "addresses", null)
  external_dns_search     = local.external_nameservers == null ? null : lookup(local.external_nameservers, "search", null)

  main_interface_configuration = jsonencode(merge(
    local.external_ip == null ? { "dhcp4" = true } : tomap({}),
    local.external_ip != null ? { "addresses" = [local.external_ip] } : tomap({}),
    local.external_gateway != null ? { "gateway4" = local.external_gateway } : tomap({}),
    local.external_nameservers != null ? {
      "nameservers" = merge(
        local.external_dns_addresses != null ? { "addresses" = local.external_dns_addresses } : tomap({}),
        local.external_dns_search != null ? { "search" = local.external_dns_search } : tomap({})
      )
    } : tomap({})
  ))

  first_interface_index     = 192

  cloud_init_network = {
    version   = 2
    ethernets = {
      "ens${local.first_interface_index}" = jsondecode(local.main_interface_configuration)
    }
  }

  cloud_init_metadata = {
    "local-hostname"   = join("-", [local.prefix, "master", var.nodeIndex])
    "public-keys-data" = var.providerClusterConfiguration.sshPublicKey
    "network"          = local.cloud_init_network
  }

  vm_extra_config_guestinfo = {
    "guestinfo.metadata"          = base64encode(jsonencode(local.cloud_init_metadata))
    "guestinfo.metadata.encoding" = "base64"
    "guestinfo.userdata"          = var.cloudConfig
    "guestinfo.userdata.encoding" = "base64"
  }
}

data "vcd_catalog" "catalog" {
  name = local.catalog
}

data "vcd_catalog_vapp_template" "template" {
  catalog_id = data.vcd_catalog.catalog.id
  name       = local.template
}

data "vcd_storage_profile" "profile" {
  name = local.master_instance_class.storageProfile
}
/*
resource "vsphere_virtual_disk" "kubernetes_data" {
  size               = 10
  datastore          = local.master_instance_class.datastore
  datacenter         = data.vsphere_dynamic.datacenter_id.inventory_path
  type               = "eagerZeroedThick"
  vmdk_path          = "deckhouse/${join("-", [var.clusterUUID, "kubernetes-data", var.nodeIndex])}.vmdk"
  create_directories = true
}

*/

resource "vcd_vm" "master" {
  name             = join("-", [local.prefix, "master", var.nodeIndex])
  vapp_template_id = data.vcd_catalog_vapp_template.template.id

  cpus = local.master_instance_class.numCPUs
  memory   = local.master_instance_class.memory
  memory_hot_add_enabled = true

  network {
    name               = "internal"
    type               = "org"
    ip_allocation_mode = local.external_ip == null ? "DHCP" : "MANUAL"
    is_primary         = true
    ip                 = split("/", local.external_ip)[0]
  }

  override_template_disk {
    bus_type        = "paravirtual"
    size_in_mb      = local.clusterConfiguration.rootDiskSizeGb
    bus_number      = 0
    unit_number     = 0
    iops            = data.vcd_storage_profile.profile.iops_settings.disk_iops_per_gb_max == 0 ? 0 : data.vcd_storage_profile.profile.iops_settings.disk_iops_per_gb_max * local.clusterConfiguration.rootDiskSizeGb
    storage_profile = local.master_instance_class.storageProfile
  }
#  dynamic "network_interface" {
#    for_each = local.additionalNetworks
#    content {
#      network_id   = data.vsphere_network.internal[network_interface.value].id
#      adapter_type = data.vsphere_virtual_machine.template.network_interface_types[0]
#    }
#  }
/*
  internal_disk {
    label            = "disk0"
    unit_number      = 0
    size             = local.master_instance_class.rootDiskSize
  }

  internal_disk {
    label        = "disk1"
    unit_number  = 1
    attach       = true
    size             = local.master_instance_class.etcdDiskSizeGb
  }

  # enable_disk_uuid = true

  nested_hv_enabled  = lookup(local.runtime_options, "nestedHardwareVirtualization", null)
  cpu_limit          = lookup(local.runtime_options, "cpuLimit", null)
  cpu_reservation    = lookup(local.runtime_options, "cpuReservation", null)
  cpu_share_count    = lookup(local.runtime_options, "cpuShares", null)
  memory_limit       = lookup(local.runtime_options, "memoryLimit", null)
  memory_reservation = lookup(local.runtime_options, "memoryReservation", null)
  memory_share_count = lookup(local.runtime_options, "memoryShares", null)

  extra_config = local.vm_extra_config

#  depends_on = [vsphere_virtual_disk.kubernetes_data]
*/
}
