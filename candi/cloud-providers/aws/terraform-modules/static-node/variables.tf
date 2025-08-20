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

variable "prefix" {
  type = string
}

variable "disable_default_security_group" {
  type = bool
  default = false
}

variable "cluster_uuid" {
  type = string
}

variable "ssh_allow_list" {
  type = any
}

variable "node_group" {
  type = any
}

variable "node_index" {
  type = number
}

variable "root_volume_size" {
  type = number
}

variable "root_volume_type" {
  type = string
}

variable "associate_public_ip_address" {
  type = bool
  default = false
}

variable "cloud_config" {
  type = any
}

variable "additional_security_groups" {
  type = list(string)
  default = []
}

variable "zones" {
  type = list(string)
}

variable "tags" {
  type = map(string)
}

locals {
  zones = sort(distinct(var.zones))
}

variable "resourceManagementTimeout" {
  type = string
  default = "10m"
}
