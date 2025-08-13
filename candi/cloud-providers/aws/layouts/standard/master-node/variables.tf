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
  root_volume_size = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskSizeGb", 50)
  root_volume_type = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskType", "gp2")
  additional_security_groups = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalSecurityGroups", [])
  actual_zones = lookup(var.providerClusterConfiguration, "zones", {}) != {} ? tolist(setintersection(data.aws_availability_zones.available.names, var.providerClusterConfiguration.zones)) : data.aws_availability_zones.available.names
  zones = lookup(var.providerClusterConfiguration.masterNodeGroup, "zones", {}) != {} ? tolist(setintersection(local.actual_zones, var.providerClusterConfiguration.masterNodeGroup["zones"])) : local.actual_zones
  tags = merge(lookup(var.providerClusterConfiguration, "tags", {}), lookup(var.providerClusterConfiguration.masterNodeGroup, "additionalTags", {}))
  ssh_allow_list = lookup(var.providerClusterConfiguration, "sshAllowList", ["0.0.0.0/0"])
  disable_default_sg = lookup(var.providerClusterConfiguration, "disableDefaultSecurityGroup", false)
}
