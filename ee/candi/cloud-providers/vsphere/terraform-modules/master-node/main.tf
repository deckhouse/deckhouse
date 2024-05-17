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
  name          = local.master_instance_class.datastore
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

data "vsphere_resource_pool" "resource_pool" {
  count         = 1
  name          = join("/", [data.vsphere_dynamic.cluster_id.inventory_path, "Resources", local.resource_pool])
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

data "vsphere_network" "main" {
  name          = local.master_instance_class.mainNetwork
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

data "vsphere_network" "internal" {
  for_each      = toset(local.additionalNetworks)
  name          = each.value
  datacenter_id = data.vsphere_dynamic.datacenter_id.id
}

data "vsphere_virtual_machine" "template" {
  name          = local.master_instance_class.template
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
    local.external_ip == null ? { "dhcp4" = true } : tomap({}),
    local.external_ip != null ? { "addresses" = [local.external_ip] } : tomap({}),
    local.external_gateway != null ? { "gateway4" = local.external_gateway } : tomap({}),
    local.external_nameservers != {} ? {
      "nameservers" = merge(
        local.external_dns_addresses != null ? { "addresses" = local.external_dns_addresses } : tomap({}),
        local.external_dns_search != null ? { "search" = local.external_dns_search } : tomap({})
      )
    } : tomap({})
  ))

  internalNodeNetworkPrefix = split("/", var.providerClusterConfiguration.internalNetworkCIDR)[1]
  first_interface_index     = 192

  additional_interface_configurations = {
    for i, v in local.additionalNetworks :
    "ens${local.first_interface_index + 32 * (i + 1)}" =>
    { addresses = [join("", [cidrhost(var.providerClusterConfiguration.internalNetworkCIDR, var.nodeIndex + 10), "/", local.internalNodeNetworkPrefix])] }
  }

  cloud_init_network = {
    version = 2
    ethernets = merge({
      "ens${local.first_interface_index}" = jsondecode(local.main_interface_configuration)
    }, local.additional_interface_configurations)
  }

  cloud_init_metadata = {
    "local-hostname"   = join("-", [local.prefix, "master", var.nodeIndex])
    "public-keys-data" = var.providerClusterConfiguration.sshPublicKey
    "network"          = local.cloud_init_network
  }

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

resource "vsphere_virtual_disk" "kubernetes_data" {
  size               = 10
  datastore          = local.master_instance_class.datastore
  datacenter         = data.vsphere_dynamic.datacenter_id.inventory_path
  type               = "eagerZeroedThick"
  vmdk_path          = "deckhouse/${join("-", [var.clusterUUID, "kubernetes-data", var.nodeIndex])}.vmdk"
  create_directories = true
}

resource "vsphere_virtual_machine" "master" {
  name             = join("-", [local.prefix, "master", var.nodeIndex])
  resource_pool_id = data.vsphere_resource_pool.resource_pool[0].id
  datastore_id     = data.vsphere_datastore.datastore.id
  folder           = var.providerClusterConfiguration.vmFolderPath

  firmware = data.vsphere_virtual_machine.template.firmware
  num_cpus = local.master_instance_class.numCPUs
  memory   = local.master_instance_class.memory
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
    size             = lookup(local.master_instance_class, "rootDiskSize", 30)
    eagerly_scrub    = false
    thin_provisioned = false
  }

  disk {
    label        = "disk1"
    unit_number  = 1
    attach       = true
    path         = vsphere_virtual_disk.kubernetes_data.vmdk_path
    datastore_id = data.vsphere_datastore.datastore.id
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

  depends_on = [vsphere_virtual_disk.kubernetes_data]

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
