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

variable "api_version" {
  default = "virtualization.deckhouse.io/v1alpha2"
  type    = string
}

variable "prefix" {
  type = string
}

variable "node_group" {
  type = string
}

variable "node_index" {
  type = string
}

variable "namespace" {
  type = string
}

variable "storage_class" {
  default = null
  type    = string
}

variable "size" {
  default = "15Gi"
  type    = string
}

variable "timeouts" {
  default = { "create" = "30m", "update" = "1m", "delete" = "1m" }
  type = object({
    create = string
    update = string
    delete = string
  })
}


locals {
  data_disk_destructive_params = {
    "kbernetesDataDisk" = {
      "storageClass" = var.storage_class
    }
  }

  data_disk_destructive_params_json      = jsonencode(local.data_disk_destructive_params)
  data_disk_destructive_params_json_hash = substr(sha256(jsonencode(local.data_disk_destructive_params_json)), 0, 6)

  data_disk_name = join("-", [var.prefix, var.node_group, "kubernetes-data", var.node_index, local.data_disk_destructive_params_json_hash])

  data_disk_annotations = {
    "last_applied_destructive_root_disk_parameters"      = local.data_disk_destructive_params_json
    "last_applied_destructive_root_disk_parameters_hash" = local.data_disk_destructive_params_json_hash
  }
}

