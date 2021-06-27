# Copyright 2021 Flant CJSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE

output "device_path" {
  value = openstack_compute_volume_attach_v2.kubernetes_data.device
}
