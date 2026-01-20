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

resource "kubernetes_manifest" "ipv4_address" {
  count = var.ipv4_address != "" ? 1 : 0
  manifest = {
    "apiVersion" = var.api_version
    "kind"       = "VirtualMachineIPAddress"
    "metadata" = {
      "name"      = local.ip_address_name
      "namespace" = var.namespace
    }
    "spec" = {
      "staticIP" = local.ipv4_address
      "type"     = local.ipv4_address_type
    }
  }
  wait {
    fields = {
      "status.phase" = "Bound"
    }
  }
  timeouts {
    create = var.timeouts.create
    update = var.timeouts.update
    delete = var.timeouts.delete
  }
}

data "kubernetes_resource" "ipv4_address" {
  api_version = var.api_version
  kind        = "VirtualMachineIPAddress"
  metadata {
    name      = local.ip_address_name
    namespace = var.namespace
  }
  depends_on = [
    kubernetes_manifest.ipv4_address
  ]
}
