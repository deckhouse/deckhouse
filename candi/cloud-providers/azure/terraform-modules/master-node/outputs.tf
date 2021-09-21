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
  value = lookup(azurerm_linux_virtual_machine.master, "id")
}

output "node_internal_ip_address" {
  value = lookup(azurerm_linux_virtual_machine.master, "private_ip_address")
}

output "master_ip_address_for_ssh" {
  value = local.enable_external_ip == false ? lookup(azurerm_linux_virtual_machine.master, "private_ip_address") : lookup(azurerm_linux_virtual_machine.master, "public_ip_address")
}

output "kubernetes_data_device_path" {
  value = azurerm_virtual_machine_data_disk_attachment.kubernetes_data.lun
}
