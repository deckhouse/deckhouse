# Copyright 2021 Flant CJSC
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
    condition     = contains(keys(var.providerClusterConfiguration), "vpcNetworkCIDR") ? cidrsubnet(var.providerClusterConfiguration.vpcNetworkCIDR, 0, 0) == var.providerClusterConfiguration.vpcNetworkCIDR : true
    error_message = "Invalid vpcNetworkCIDR in AWSClusterConfiguration."
  }

  validation {
    condition     = contains(keys(var.providerClusterConfiguration), "nodeNetworkCIDR") ? cidrsubnet(var.providerClusterConfiguration.nodeNetworkCIDR, 0, 0) == var.providerClusterConfiguration.nodeNetworkCIDR : true
    error_message = "Invalid nodeNetworkCIDR in AWSClusterConfiguration."
  }
}

variable "clusterUUID" {
  type = string
}

variable "resourceManagementTimeout" {
  type = string
  default = "10m"
}

locals {
  with_nat                 = lookup(var.providerClusterConfiguration, "withNAT", {})
  bastion_instance         = lookup(local.with_nat, "bastionInstance", {})
  prefix                   = var.clusterConfiguration.cloud.prefix
  vpc_network_cidr         = lookup(var.providerClusterConfiguration, "vpcNetworkCIDR", "")
  existing_vpc_id          = lookup(var.providerClusterConfiguration, "existingVPCID", "")
  tags                     = lookup(var.providerClusterConfiguration, "tags", {})
  ssh_allow_list           = lookup(var.providerClusterConfiguration, "sshAllowList", ["0.0.0.0/0"])
  additional_role_policies = lookup(var.providerClusterConfiguration, "additionalRolePolicies", [])
  disable_default_sg = lookup(var.providerClusterConfiguration, "disableDefaultSecurityGroup", false)
}
