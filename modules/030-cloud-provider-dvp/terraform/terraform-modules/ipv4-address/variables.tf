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

variable "namespace" {
  type = string
}

variable "hostname" {
  type = string
}

variable "ipv4_address" {
  type = string
}

variable "timeouts" {
  default = { "create" = "10m", "update" = "5m", "delete" = "5m" }
  type = object({
    create = string
    update = string
    delete = string
  })
}

locals {

  ipv4_address_type = var.ipv4_address == "Auto" ? "Auto" : "Static"
  ipv4_address      = var.ipv4_address == "Auto" ? "" : var.ipv4_address
  ip_address_name   = lower(join("-", [var.hostname, replace(var.ipv4_address, ".", "-")]))
}
