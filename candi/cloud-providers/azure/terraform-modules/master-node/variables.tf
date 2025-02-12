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
    condition     = replace(var.providerClusterConfiguration.masterNodeGroup.instanceClass.urn, "latest", "") == var.providerClusterConfiguration.masterNodeGroup.instanceClass.urn ? true : false
    error_message = "Not allowed to use latest as image version."
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

variable "resourceManagementTimeout" {
  type = string
  default = "10m"
}

locals {
  prefix                 = var.clusterConfiguration.cloud.prefix
  admin_username         = "azureuser"
  machine_size           = var.providerClusterConfiguration.masterNodeGroup.instanceClass.machineSize
  ssh_public_key         = var.providerClusterConfiguration.sshPublicKey
  disk_type              = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskType", "StandardSSD_LRS")
  disk_size_gb           = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskSizeGb", 50)
  etcd_disk_size_gb      = var.providerClusterConfiguration.masterNodeGroup.instanceClass.etcdDiskSizeGb
  enable_external_ip     = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "enableExternalIP", false)
  urn                    = split(":", var.providerClusterConfiguration.masterNodeGroup.instanceClass.urn)
  image_publisher        = local.urn[0]
  image_offer            = local.urn[1]
  image_sku              = local.urn[2]
  image_version          = local.urn[3]
  actual_zones           = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(["1", "2", "3"], var.providerClusterConfiguration.zones)) : ["1", "2", "3"]
  zones                  = lookup(var.providerClusterConfiguration.masterNodeGroup, "zones", null) != null ? tolist(setintersection(local.actual_zones, var.providerClusterConfiguration.masterNodeGroup["zones"])) : local.actual_zones
  additional_tags        = merge(lookup(var.providerClusterConfiguration, "tags", {}), lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalTags", {}))
  accelerated_networking = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "acceleratedNetworking", false)
}
