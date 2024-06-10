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
  type    = string
  default = ""
}

variable "clusterUUID" {
  type = string
}

variable "network_types" {
  type = map(any)
  default = {
    "Standard"            = "standard"
    "SoftwareAccelerated" = "software_accelerated"
  }
}

locals {
  prefix            = var.clusterConfiguration.cloud.prefix
  ng                = [for i in var.providerClusterConfiguration.nodeGroups : i if i.name == var.nodeGroupName][0]
  instance_class    = local.ng["instanceClass"]
  platform          = lookup(local.instance_class, "platform", "standard-v2")
  cores             = local.instance_class.cores
  core_fraction     = lookup(local.instance_class, "coreFraction", null)
  memory            = local.instance_class.memory / 1024
  disk_size_gb      = lookup(local.instance_class, "diskSizeGB", 50)
  image_id          = local.instance_class.imageID
  node_network_cidr = var.providerClusterConfiguration.nodeNetworkCIDR
  ssh_public_key    = var.providerClusterConfiguration.sshPublicKey

  external_subnet_ids           = lookup(local.instance_class, "externalSubnetIDs", [])
  external_ip_addresses         = lookup(local.instance_class, "externalIPAddresses", [])
  external_subnet_id_deprecated = lookup(local.instance_class, "externalSubnetID", null)

  network_type      = contains(keys(local.instance_class), "networkType") ? var.network_types[local.instance_class.networkType] : null
  additional_labels = merge(lookup(var.providerClusterConfiguration, "labels", {}), lookup(local.instance_class, "additionalLabels", {}))
}
