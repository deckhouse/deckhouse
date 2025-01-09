# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

resource "openstack_blockstorage_volume_v3" "kubernetes_data" {
  name = join("-", [var.prefix, "kubernetes-data", var.node_index])
  description = "volume for etcd and kubernetes certs"
  size = var.volume_size
  volume_type = var.volume_type
  availability_zone = var.volume_zone
  enable_online_resize = true
  metadata = var.tags

  lifecycle {
    ignore_changes = [
      metadata,
      availability_zone,
    ]
  }

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
  }
}

resource "openstack_compute_volume_attach_v2" "kubernetes_data" {
  instance_id = var.master_id
  volume_id = openstack_blockstorage_volume_v3.kubernetes_data.id
}
