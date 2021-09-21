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

output "id" {
  value = lookup(google_compute_instance.master, "instance_id")
}

output "node_internal_ip_address" {
  value = google_compute_instance.master.network_interface.0.network_ip
}

output "master_ip_address_for_ssh" {
  value = local.disable_external_ip == true ? google_compute_instance.master.network_interface.0.network_ip : google_compute_instance.master.network_interface.0.access_config.0.nat_ip
}

output "kubernetes_data_device_path" {
  value = google_compute_instance.master.attached_disk.0.device_name
}
