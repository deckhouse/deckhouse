# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "device_path" {
  value = openstack_compute_volume_attach_v2.kubernetes_data.device
}
