# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

resource "vcd_vapp" "vapp" {
  name     = var.vapp_name
  org      = var.organization
  power_on = true

  dynamic "metadata_entry" {
    for_each = var.metadata

    content {
      type        = "MetadataStringValue"
      is_system   = false
      user_access = "READWRITE"
      key         = metadata_entry.key
      value       = metadata_entry.value
    }
  }
}
