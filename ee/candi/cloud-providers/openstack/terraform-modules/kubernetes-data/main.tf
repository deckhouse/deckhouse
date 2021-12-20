# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

resource "openstack_blockstorage_volume_v2" "kubernetes_data" {
  name = join("-", [var.prefix, "kubernetes-data", var.node_index])
  description = "volume for etcd and kubernetes certs"
  size = 10
  volume_type = var.volume_type
  availability_zone = var.volume_zone
  metadata = var.tags
  lifecycle {
    ignore_changes = [
      metadata,
    ]
  }
}

resource "openstack_compute_volume_attach_v2" "kubernetes_data" {
  instance_id = var.master_id
  volume_id = openstack_blockstorage_volume_v2.kubernetes_data.id
}
