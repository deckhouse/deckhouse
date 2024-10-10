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
    condition     = contains(keys(var.providerClusterConfiguration), "vNetCIDR") ? cidrsubnet(var.providerClusterConfiguration.vNetCIDR, 0, 0) == var.providerClusterConfiguration.vNetCIDR : true
    error_message = "Invalid vNetCIDR in AzureClusterConfiguration."
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
  prefix                      = var.clusterConfiguration.cloud.prefix
  location                    = var.providerClusterConfiguration.provider.location
  vnet_cidr                   = var.providerClusterConfiguration.vNetCIDR
  subnet_cidr                 = var.providerClusterConfiguration.subnetCIDR
  zones                       = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(["1", "2", "3"], var.providerClusterConfiguration.zones)) : ["1", "2", "3"]
  peered_vnets                = { for vpc in lookup(var.providerClusterConfiguration, "peeredVNets", []) : vpc.vnetName => vpc }
  enable_nat_gateway          = lookup(var.providerClusterConfiguration, "enableNatGateway", false)
  additional_tags             = lookup(var.providerClusterConfiguration, "tags", {})
  nat_gateway_public_ip_count = contains(keys(var.providerClusterConfiguration), "standard") ? lookup(var.providerClusterConfiguration.standard, "natGatewayPublicIpCount", 0) : 0
  ssh_allow_list              = lookup(var.providerClusterConfiguration, "sshAllowList", null)
  service_endpoints           = lookup(var.providerClusterConfiguration, "serviceEndpoints", [])
  nameservers                 = lookup(var.providerClusterConfiguration.nameservers, "addresses", null)
}
