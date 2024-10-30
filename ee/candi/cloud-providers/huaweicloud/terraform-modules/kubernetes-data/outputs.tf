# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "device_path" {
  value = huaweicloud_compute_volume_attach.kubernetes_data.device
}
