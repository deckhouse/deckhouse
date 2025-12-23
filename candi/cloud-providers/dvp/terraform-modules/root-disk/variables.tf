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

variable "owner_ref_kind" {
  default = "VirtualMachine"
  type = string
}

variable "owner_ref_name" {
  type = string
}

variable "owner_ref_uid" {
  type = string
}

variable "root_disk_destructive_params_json" {
  type = string
}

variable "root_disk_destructive_params_json_hash" {
  type = string
}

variable "root_disk_name" {
  type = string
}

variable "namespace" {
  type = string
}

variable "storage_class" {
  default = null
  type    = string
}

variable "image" {
  type = object({
    kind = string
    name = string
  })
}

variable "size" {
  default = "50Gi"
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
  root_disk_annotations = {
    "last_applied_destructive_root_disk_parameters"      = var.root_disk_destructive_params_json
    "last_applied_destructive_root_disk_parameters_hash" = var.root_disk_destructive_params_json_hash
  }
}
