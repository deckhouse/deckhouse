# Copyright 2024 Flant JSC
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

data "kubernetes_resource" "vm_data" {
  api_version = local.apiVersion
  kind        = "VirtualMachine"

  metadata {
    name      = local.vm_name
    namespace = local.namespace
  }
  depends_on = [
    kubernetes_manifest.vm
  ]

}

output "master_ip_address_for_ssh" {
  value = data.kubernetes_resource.vm_data.object.status.ipAddress
}

output "node_internal_ip_address" {
  value = data.kubernetes_resource.vm_data.object.status.ipAddress
}
