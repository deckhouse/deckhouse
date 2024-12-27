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

variable "resourceManagementTimeout" {
  type = string
  default = "10m"
}

locals {
  prefix                       = var.clusterConfiguration.cloud.prefix
  machine_type                 = var.providerClusterConfiguration.masterNodeGroup.instanceClass.machineType
  image                        = var.providerClusterConfiguration.masterNodeGroup.instanceClass.image
  disk_size_gb                 = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskSizeGb", 50)
  etcd_disk_size_gb            = var.providerClusterConfiguration.masterNodeGroup.instanceClass.etcdDiskSizeGb
  ssh_key                      = var.providerClusterConfiguration.sshKey
  ssh_user                     = "user"
  disable_external_ip          = var.providerClusterConfiguration.layout == "WithoutNAT" ? false : lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "disableExternalIP", true)
  actual_zones                 = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.google_compute_zones.available.names, var.providerClusterConfiguration.zones)) : data.google_compute_zones.available.names
  zones                        = lookup(var.providerClusterConfiguration.masterNodeGroup, "zones", null) != null ? tolist(setintersection(local.actual_zones, var.providerClusterConfiguration.masterNodeGroup["zones"])) : local.actual_zones
  additional_network_tags      = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalNetworkTags", [])
  service_account_client_email = jsondecode(var.providerClusterConfiguration.provider.serviceAccountJSON).client_email
  additional_labels            = merge(lookup(var.providerClusterConfiguration, "labels", {}), lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalLabels", null))
}
