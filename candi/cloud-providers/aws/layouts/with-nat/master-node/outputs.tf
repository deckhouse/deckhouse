# Copyright 2021 Flant CJSC
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

output "master_ip_address_for_ssh" {
  value = module.master-node.master_private_ip
}

output "node_internal_ip_address" {
  value = module.master-node.master_private_ip
}

output "kubernetes_data_device_path" {
  value = module.master-node.kubernetes_data_device_path
}

output "system_registry_data_device_path" {
  value = module.master-node.system_registry_data_device_path
}

