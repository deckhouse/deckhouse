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

variable "dhcp_domain_name" {
  type = string
  default = null
}

variable "dhcp_domain_name_servers" {
  type = list(string)
  default = null
}

variable "network_id" {
  type = string
}

variable "node_network_cidr" {
  type = string
  default = null
}

variable "existing_zone_to_subnet_id_map" {
  type = map
  default = {}
}

variable "layout" {
  type = string
}

variable "nat_instance_external_address" {
  type = string
  default = null
}

variable "nat_instance_internal_address" {
  type = string
  default = null
}

variable "nat_instance_internal_subnet_id" {
  type = string
  default = null
}

variable "nat_instance_external_subnet_id" {
  type = string
  default = null
}

variable "nat_instance_cores" {
  type = number
  default = null
}

variable "nat_instance_memory" {
  type = number
  default = null
}

variable "nat_instance_ssh_key" {
  type = string
  default = ""
}

variable "labels" {
  type = map
}
