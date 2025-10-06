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

resource "kubernetes_manifest" "kubernetes-data-disk" {
  manifest = {
    "apiVersion" = var.api_version
    "kind"       = "VirtualDisk"
    "metadata" = {
      "name"        = local.data_disk_name
      "namespace"   = var.namespace
      "annotations" = local.data_disk_annotations
    }
    "spec" = {
      "persistentVolumeClaim" = merge({
        "size" = var.size
        },
        var.storage_class != null ? { "storageClassName" = var.storage_class } : null
      )
    }
  }
  timeouts {
    create = var.timeouts.create
    update = var.timeouts.update
    delete = var.timeouts.delete
  }
  lifecycle {
    ignore_changes = [
      object.spec.persistentVolumeClaim.storageClassName
    ]
  }
}

data "kubernetes_resource" "kubernetes-data-disk" {
  api_version = var.api_version
  kind        = "VirtualDisk"

  metadata {
    name      = local.data_disk_name
    namespace = var.namespace
  }
  depends_on = [
    kubernetes_manifest.kubernetes-data-disk
  ]
}
