# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "zone" {
  value = contains([for zone in data.huaweicloud_evs_availability_zones.zones.availability_zones: zone.name if zone.is_available], var.compute_zone) ? var.compute_zone : null
}
