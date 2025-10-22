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
}

variable "nodeGroupName" {
  type = string
}

variable "nodeIndex" {
  type = number
}

variable "cloudConfig" {
  type = any
  default = ""
}

variable "clusterUUID" {
  type = string
}

variable "resourceManagementTimeout" {
  type = string
  default = "10m"
}

data "aws_availability_zones" "available" {}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  node_groups = lookup(var.providerClusterConfiguration, "nodeGroups", [])
  node_group = [for i in local.node_groups: i if i.name == var.nodeGroupName][0]
  root_volume_size = lookup(local.node_group.instanceClass, "diskSizeGb", 50)
  root_volume_type = lookup(local.node_group.instanceClass, "diskType", "gp2")
  additional_security_groups = lookup(local.node_group.instanceClass, "additionalSecurityGroups", [])
  actual_zones = lookup(var.providerClusterConfiguration, "zones", {}) != {} ? tolist(setintersection(data.aws_availability_zones.available.names, var.providerClusterConfiguration.zones)) : data.aws_availability_zones.available.names
  zones = lookup(local.node_group, "zones", {}) != {} ? tolist(setintersection(local.actual_zones, local.node_group["zones"])) : local.actual_zones
  tags = merge(lookup(var.providerClusterConfiguration, "tags", {}), lookup(local.node_group, "additionalTags", {}))
  disable_default_sg       = lookup(var.providerClusterConfiguration, "disableDefaultSecurityGroup", false)
  ssh_allow_list           = lookup(var.providerClusterConfiguration, "sshAllowList", ["0.0.0.0/0"])
}
