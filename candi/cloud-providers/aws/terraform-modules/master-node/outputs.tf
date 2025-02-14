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

output "master_public_ip" {
  value = var.associate_public_ip_address ? join("", aws_eip.eip.*.public_ip) : ""
}

output "master_private_ip" {
  value = aws_instance.master.private_ip
}

output "kubernetes_data_device_path" {
  value = aws_volume_attachment.kubernetes_data.device_name
}

output "system_registry_data_device_path" {
  value = var.registryDataDeviceEnable ? aws_volume_attachment.system_registry_data[0].device_name : ""
}
