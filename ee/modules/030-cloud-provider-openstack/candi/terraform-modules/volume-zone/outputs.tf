# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "zone" {
  value = contains(data.openstack_blockstorage_availability_zones_v3.zones.names, var.compute_zone) ? var.compute_zone : null
}
