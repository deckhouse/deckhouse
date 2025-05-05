# Copyright 2025 Flant JSC
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
  value = data.kubernetes_resource.vm_data.object.status.ipAddress
}

output "node_internal_ip_address" {
  value = data.kubernetes_resource.vm_data.object.status.ipAddress
}

output "kubernetes_data_device_path" {
  # vd-${disk-name}
  value = "/dev/disk/by-id/scsi-SQEMU_QEMU_HARDDISK_${var.kubernetes_data_disk.md5_id}"
}

