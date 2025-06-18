# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

resource "huaweicloud_evs_volume" "kubernetes_data" {
  name              = join("-", [var.prefix, "kubernetes-data", var.node_index])
  description       = "volume for etcd and kubernetes certs"
  size              = var.volume_size
  volume_type       = var.volume_type
  availability_zone = var.volume_zone
  tags              = var.tags
  enterprise_project_id = var.enterprise_project_id
  lifecycle {
    ignore_changes = [
      tags,
      availability_zone,
    ]
  }
}

resource "huaweicloud_compute_volume_attach" "kubernetes_data" {
  instance_id = var.master_id
  volume_id   = huaweicloud_evs_volume.kubernetes_data.id
}
