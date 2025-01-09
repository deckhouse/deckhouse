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

variable "nodeGroupName" {
  type = string
}

variable "clusterUUID" {
  type = string
}

variable "resourceManagementTimeout" {
  type = string
  default = "10m"
}

locals {
  prefix                       = var.clusterConfiguration.cloud.prefix
  node_groups                  = lookup(var.providerClusterConfiguration, "nodeGroups", [])
  node_group                   = [for i in local.node_groups : i if i.name == var.nodeGroupName][0]
  node_group_name              = local.node_group.name
  machine_type                 = local.node_group.instanceClass.machineType
  image                        = local.node_group.instanceClass.image
  disk_size_gb                 = lookup(local.node_group.instanceClass, "diskSizeGb", 50)
  disk_type                    = lookup(local.node_group.instanceClass, "diskType", "pd-ssd")
  ssh_key                      = var.providerClusterConfiguration.sshKey
  ssh_user                     = "user"
  disable_external_ip          = var.providerClusterConfiguration.layout == "WithoutNAT" ? false : lookup(local.node_group.instanceClass, "disableExternalIP", true)
  actual_zones                 = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.google_compute_zones.available.names, var.providerClusterConfiguration.zones)) : data.google_compute_zones.available.names
  zones                        = lookup(local.node_group, "zones", null) != null ? tolist(setintersection(local.actual_zones, local.node_group["zones"])) : local.actual_zones
  additional_network_tags      = lookup(local.node_group.instanceClass, "additionalNetworkTags", [])
  service_account_client_email = jsondecode(var.providerClusterConfiguration.provider.serviceAccountJSON).client_email
  additional_labels            = merge(lookup(var.providerClusterConfiguration, "labels", {}), lookup(local.node_group.instanceClass, "additionalLabels", {}))
}
