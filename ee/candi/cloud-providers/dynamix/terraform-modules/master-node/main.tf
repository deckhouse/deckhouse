# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "decort_account" "account" {
   account_id = local.account_id
}

data "decort_image" "os_image" {
  image_id = local.os_image_id
}

resource "decort_resgroup" "resource_group" {
  name = local.resource_group_name
  account_id = data.decort_account.account.account_id
  gid = local.grid
}

resource "decort_disk" "kubernetes_data_disk" {
   disk_name = local.kubernetes_data_disk_name
   account_id = data.decort_account.account.account_id
   gid = local.grid
   size_max = local.master_etcd_disk_size
   type = "D"    # disk type, always use "D" for extra disks
   sep_id = data.decort_image.os_image.sep_id
   pool = local.pool
}

resource "decort_cb_kvmvm" "master_vm" {
  name = local.master_node_name
  driver = local.driver
  rg_id = decort_resgroup.resource_group.id
  cpu = local.master_cpus
  ram = local.master_ram_mb
  boot_disk_size = local.master_root_disk_size
  image_id = data.decort_image.os_image.image_id
  pool = local.pool
  extra_disks = [ decort_disk.kubernetes_data_disk.id ]

  network {
    net_type = local.net_type_extnet
    net_id = local.extnet_id
  }
  network {
    net_type = local.net_type_vins
    net_id = local.vins_id
  }
}

