# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "decort_locations_list" "locations" {
  name = local.location
}

data "decort_image_list" "images" {
  name = local.image_name
}

data "decort_rg_list" "resource_group" {
  name = local.resource_group_name
}

data "decort_vins_list" "vins" {
  name = local.vins_name
}

data "decort_extnet_list" "extnets" {
  name = local.extnet_name
}

data "decort_cb_sep_list" "storage_endpoints" {
  name = local.storage_endpoint
}

locals {
  gid                 = data.decort_locations_list.locations.items[0].gid
  account_id          = data.decort_rg_list.resource_group.items[0].account_id
  image_id            = data.decort_image_list.images.items[0].image_id
  rg_id               = data.decort_rg_list.resource_group.items[0].rg_id
  extnet_id           = data.decort_extnet_list.extnets.items[0].net_id
  storage_endpoint_id = data.decort_cb_sep_list.storage_endpoints.items[0].sep_id
}

resource "decort_disk" "kubernetes_data_disk" {
  disk_name  = local.kubernetes_data_disk_name
  account_id = local.account_id
  gid        = local.gid
  size_max   = local.master_etcd_disk_size
  type       = "D" # disk type, always use "D" for extra disks
  sep_id     = local.storage_endpoint_id
  pool       = local.pool

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
    update = var.resourceManagementTimeout
  }
}

resource "decort_kvmvm" "master_vm" {
  name           = local.master_node_name
  driver         = local.driver
  rg_id          = local.rg_id
  cpu            = local.master_cpus
  ram            = local.master_ram_mb
  boot_disk_size = local.master_root_disk_size
  image_id       = local.image_id
  pool           = local.pool
  extra_disks    = [decort_disk.kubernetes_data_disk.id]
  cloud_init     = local.master_cloud_init_script

  dynamic "network" {
    for_each = length(data.decort_vins_list.vins.items) > 0 ? [data.decort_vins_list.vins.items[0].vins_id] : []
    content {
      net_type = local.net_type_vins
      net_id   = network.value
    }
  }

  network {
    net_type = local.net_type_extnet
    net_id   = local.extnet_id
  }

  lifecycle {
    ignore_changes = [
      cloud_init,
    ]
  }

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
    update = var.resourceManagementTimeout
  }
}
