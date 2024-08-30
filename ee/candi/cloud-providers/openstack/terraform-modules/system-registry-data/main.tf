# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

resource "openstack_blockstorage_volume_v3" "system_registry_data" {
  name = join("-", [var.prefix, "system-registry-data", var.node_index])
  description = "volume for system registry data"
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
}

resource "openstack_compute_volume_attach_v2" "system_registry_data" {
  instance_id = var.master_id
  volume_id = openstack_blockstorage_volume_v3.system_registry_data.id
}
