# Copyright 2021 Flant JSC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

locals {
  master_internal_ip_iface_index = local.external_subnet_id != null ? 1 : 0
}

output "master_ip_address_for_ssh" {
  value = lookup(yandex_compute_instance.master.network_interface.0, "nat_ip_address", "") != "" ? yandex_compute_instance.master.network_interface.0.nat_ip_address : yandex_compute_instance.master.network_interface.0.ip_address
}

output "node_internal_ip_address" {
  value = yandex_compute_instance.master.network_interface[local.master_internal_ip_iface_index].ip_address
}

output "kubernetes_data_device_path" {
  value = "/dev/disk/by-id/virtio-kubernetes-data"
}
