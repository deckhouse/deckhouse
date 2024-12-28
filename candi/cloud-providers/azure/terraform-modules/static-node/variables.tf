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

variable "nodeGroupName" {
  type = string
}

variable "resourceManagementTimeout" {
  type = string
  default = "10m"
}

locals {
  prefix                 = var.clusterConfiguration.cloud.prefix
  admin_username         = "azureuser"
  ssh_public_key         = var.providerClusterConfiguration.sshPublicKey
  node_groups            = lookup(var.providerClusterConfiguration, "nodeGroups", [])
  node_group             = [for i in local.node_groups : i if i.name == var.nodeGroupName][0]
  node_group_name        = local.node_group.name
  machine_size           = local.node_group.instanceClass.machineSize
  disk_type              = lookup(local.node_group.instanceClass, "diskType", "StandardSSD_LRS")
  disk_size_gb           = lookup(local.node_group.instanceClass, "diskSizeGb", 50)
  enable_external_ip     = lookup(local.node_group.instanceClass, "enableExternalIP", false)
  urn                    = split(":", local.node_group.instanceClass.urn)
  image_publisher        = local.urn[0]
  image_offer            = local.urn[1]
  image_sku              = local.urn[2]
  image_version          = local.urn[3]
  actual_zones           = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(["1", "2", "3"], var.providerClusterConfiguration.zones)) : ["1", "2", "3"]
  zones                  = lookup(local.node_group, "zones", null) != null ? tolist(setintersection(local.actual_zones, local.node_group["zones"])) : local.actual_zones
  additional_tags        = merge(lookup(var.providerClusterConfiguration, "tags", {}), lookup(local.node_group.instanceClass, "additionalTags", {}))
  accelerated_networking = lookup(local.node_group.instanceClass, "acceleratedNetworking", false)
}
