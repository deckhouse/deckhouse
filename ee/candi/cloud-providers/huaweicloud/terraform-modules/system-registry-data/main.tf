# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

resource "huaweicloud_evs_volume" "system_registry_data" {
  name              = join("-", [var.prefix, "system-registry-data", var.node_index])
  description       = "volume for system registry data"
  size              = var.volume_size
  volume_type       = var.volume_type
  availability_zone = var.volume_zone
  tags              = var.tags
  lifecycle {
    ignore_changes = [
      tags,
      availability_zone,
    ]
  }
}

resource "huaweicloud_compute_volume_attach" "system_registry_data" {
  instance_id = var.master_id
  volume_id   = huaweicloud_evs_volume.system_registry_data.id
}
