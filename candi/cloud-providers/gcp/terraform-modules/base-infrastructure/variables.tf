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

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
  validation {
    condition     = contains(keys(var.providerClusterConfiguration), "subnetworkCIDR") ? cidrsubnet(var.providerClusterConfiguration.subnetworkCIDR, 0, 0) == var.providerClusterConfiguration.subnetworkCIDR : true
    error_message = "Invalid subnetworkCIDR in GCPClusterConfiguration."
  }
}

variable "nodeIndex" {
  type    = string
  default = ""
}

variable "cloudConfig" {
  type    = string
  default = ""
}

variable "clusterUUID" {
  type = string
}

locals {
  prefix              = var.clusterConfiguration.cloud.prefix
  pod_subnet_cidr     = lookup(var.clusterConfiguration, "podSubnetCIDR", "10.100.0.0/16")
  subnetwork_cidr     = lookup(var.providerClusterConfiguration, "subnetworkCIDR", "10.172.0.0/16")
  cloud_nat_addresses = var.providerClusterConfiguration.layout == "Standard" ? lookup(lookup(var.providerClusterConfiguration, "standard", {}), "cloudNATAddresses", []) : []
  peered_vpcs_names   = lookup(var.providerClusterConfiguration, "peeredVPCs", [])
  ssh_allow_list      = lookup(var.providerClusterConfiguration, "sshAllowList", ["0.0.0.0/0"])
}
