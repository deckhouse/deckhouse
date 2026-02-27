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
      "labels"      = local.data_disk_labels
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

# WARNING! if you change this resource and list please
# evaluate to change same resource and list in:
#   ../additional-disk/variables.tf
#   ../root-disk/variables.tf
# opentofu does not support call sibling modules and we cannot use this resource in root
locals {
  not_ready_fail_reasons = [
    "ImageNotReady",
    "ClusterImageNotReady",
    "ImageNotFound",
    "ClusterImageNotFound",
    "ProvisioningFailed",
    "PVCLost",
    "QuotaExceeded",
    "ImagePullFailed",
    "DatasourceIsNotReady",
    "DatasourceIsNotFound"
  ]
}

resource "kubernetes_resource_ready_v1" "kubernetes-data-disk" {
  api_version = kubernetes_manifest.kubernetes-data-disk.object.apiVersion
  kind = kubernetes_manifest.kubernetes-data-disk.object.kind
  name = kubernetes_manifest.kubernetes-data-disk.object.metadata.name
  namespace = kubernetes_manifest.kubernetes-data-disk.object.metadata.namespace

  wait_timeout = var.timeouts.create
  # todo this attribute used on migration to resource ready resource
  # and not check ready when converge
  # it can safe delete in future because any change this attribute not produce new plan
  # 120h = 5 days
  skip_check_on_create_with_resource_live_time = "120h"

  fields = {
    # use wildcard for always present field for using fail conditions
    # resource ready resource require fields or conditions
    "metadata.name" = ".+"
  }

  fail_condition {
    type = "Ready"
    status = "False"
    reason = format("^(%s)$", join("|", local.not_ready_fail_reasons))
  }

  # wait 15 seconds appearance of the conditions to fail fast
  fail_conditions_appearance_duration = "15s"
}

data "kubernetes_resource" "kubernetes-data-disk" {
  api_version = var.api_version
  kind        = "VirtualDisk"

  metadata {
    name      = local.data_disk_name
    namespace = var.namespace
  }
  depends_on = [
    # wait to disk is ready
    kubernetes_resource_ready_v1.kubernetes-data-disk,
    kubernetes_manifest.kubernetes-data-disk
  ]
}
