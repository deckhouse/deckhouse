# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "vsphere_tag_category" "region" {
  name = var.providerClusterConfiguration.regionTagCategory
}

data "vsphere_tag_category" "zone" {
  name = var.providerClusterConfiguration.zoneTagCategory
}

data "vsphere_tag" "region_tag" {
  name        = var.providerClusterConfiguration.region
  category_id = data.vsphere_tag_category.region.id
}

data "vsphere_dynamic" "datacenter_id" {
  filter                 = [data.vsphere_tag.region_tag.id]
  type                   = "Datacenter"
  resolve_inventory_path = true
}

data "vsphere_tag" "zone_tag" {
  name        = local.zone
  category_id = data.vsphere_tag_category.zone.id
}

data "vsphere_dynamic" "cluster_id" {
  filter                 = [data.vsphere_tag.zone_tag.id]
  type                   = "ClusterComputeResource"
  resolve_inventory_path = true
}

data "vsphere_datastore" "datastore" {
  name          = local.instance_class.datastore
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

data "vsphere_resource_pool" "resource_pool" {
  count         = 1
  name          = join("/", [data.vsphere_dynamic.cluster_id.inventory_path, "Resources", local.resource_pool])
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

data "vsphere_network" "main" {
  name          = local.instance_class.mainNetwork
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

data "vsphere_network" "internal" {
  for_each      = toset(local.additionalNetworks)
  name          = each.value
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

data "vsphere_virtual_machine" "template" {
  name          = local.instance_class.template
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

locals {
  external_addresss      = length(local.main_ip_addresses) > 0 ? element(local.main_ip_addresses, var.nodeIndex) : tomap({})
  external_ip            = lookup(local.external_addresss, "address", null)
  external_gateway       = lookup(local.external_addresss, "gateway", null)
  external_nameservers   = lookup(local.external_addresss, "nameservers", {})
  external_dns_addresses = lookup(local.external_nameservers, "addresses", null)
  external_dns_search    = lookup(local.external_nameservers, "search", null)

  main_interface_configuration = jsonencode(merge(
    local.external_ip != null ? { "addresses" = [local.external_ip] } : tomap({}),
    local.external_gateway != null ? { "gateway4" = local.external_gateway } : tomap({}),
    local.external_nameservers != {} ? {
      "nameservers" = merge(
        local.external_dns_addresses != null ? { "addresses" = local.external_dns_addresses } : tomap({}),
        local.external_dns_search != null ? { "search" = local.external_dns_search } : tomap({})
      )
    } : tomap({})
  ))

  first_interface_index = 192

  cloud_init_network = {
    version = 2
    ethernets = {
      "ens${local.first_interface_index}" = jsondecode(local.main_interface_configuration)
    }
  }

  cloud_init_metadata = merge({
    "local-hostname"   = join("-", [local.prefix, local.node_group_name, var.nodeIndex])
    "public-keys-data" = var.providerClusterConfiguration.sshPublicKey
  }, local.external_ip != null ? { "network" = local.cloud_init_network } : {})

  timesync_extra_conf = lookup(var.providerClusterConfiguration, "disableTimesync", true) ? {
    "time.synchronize.continue"      = "0"
    "time.synchronize.restore"       = "0"
    "time.synchronize.resume.disk"   = "FALSE"
    "time.synchronize.shrink"        = "0"
    "time.synchronize.tools.startup" = "FALSE"
    "time.synchronize.tools.enable"  = "FALSE"
    "time.synchronize.resume.host"   = "0"
  } : {}

  vm_extra_config_guestinfo = {
    "guestinfo.metadata"          = base64encode(jsonencode(local.cloud_init_metadata))
    "guestinfo.metadata.encoding" = "base64"
    "guestinfo.userdata"          = var.cloudConfig
    "guestinfo.userdata.encoding" = "base64"
  }

  vm_extra_config = merge(local.timesync_extra_conf, local.vm_extra_config_guestinfo)
}

resource "vsphere_virtual_machine" "node" {
  name             = join("-", [local.prefix, local.node_group_name, var.nodeIndex])
  resource_pool_id = data.vsphere_resource_pool.resource_pool[0].id
  datastore_id     = data.vsphere_datastore.datastore.id
  folder           = var.providerClusterConfiguration.vmFolderPath

  firmware = data.vsphere_virtual_machine.template.firmware
  num_cpus = local.instance_class.numCPUs
  memory   = local.instance_class.memory
  guest_id = data.vsphere_virtual_machine.template.guest_id

  scsi_type = data.vsphere_virtual_machine.template.scsi_type

  network_interface {
    network_id   = data.vsphere_network.main.id
    adapter_type = data.vsphere_virtual_machine.template.network_interface_types[0]
  }

  dynamic "network_interface" {
    for_each = local.additionalNetworks
    content {
      network_id   = data.vsphere_network.internal[network_interface.value].id
      adapter_type = data.vsphere_virtual_machine.template.network_interface_types[0]
    }
  }

  disk {
    label            = "disk0"
    unit_number      = 0
    size             = lookup(local.instance_class, "rootDiskSize", 30)
    eagerly_scrub    = false
    thin_provisioned = false
  }

  enable_disk_uuid = true

  nested_hv_enabled  = lookup(local.runtime_options, "nestedHardwareVirtualization", null)
  cpu_limit          = lookup(local.runtime_options, "cpuLimit", null)
  cpu_reservation    = lookup(local.runtime_options, "cpuReservation", null)
  cpu_share_count    = lookup(local.runtime_options, "cpuShares", null)
  memory_limit       = lookup(local.runtime_options, "memoryLimit", null)
  memory_reservation = lookup(local.runtime_options, "memoryReservation", null)
  memory_share_count = lookup(local.runtime_options, "memoryShares", null)

  extra_config = local.vm_extra_config

  clone {
    template_uuid = data.vsphere_virtual_machine.template.id
  }

  cdrom {
    client_device = true
  }

  lifecycle {
    ignore_changes = [
      extra_config,
      disk,
      vapp,
      firmware,
    ]
  }
  wait_for_guest_net_routable = var.wait_for_guest_net_routable
}
