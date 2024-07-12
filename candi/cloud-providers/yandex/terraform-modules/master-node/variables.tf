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
  type    = number
  default = 0
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
  prefix                = var.clusterConfiguration.cloud.prefix
  mng                   = var.providerClusterConfiguration.masterNodeGroup
  master_instance_class = var.providerClusterConfiguration.masterNodeGroup.instanceClass
  platform              = lookup(local.master_instance_class, "platform", "standard-v2")
  cores                 = local.master_instance_class.cores
  memory                = local.master_instance_class.memory / 1024
  disk_size_gb          = lookup(local.master_instance_class, "diskSizeGB", 50)
  disk_type             = lookup(local.master_instance_class, "diskType", "network-ssd")
  etcd_disk_size_gb     = local.master_instance_class.etcdDiskSizeGb
  image_id              = local.master_instance_class.imageID
  ssh_public_key        = var.providerClusterConfiguration.sshPublicKey
  node_network_cidr     = var.providerClusterConfiguration.nodeNetworkCIDR

  external_ip_addresses         = lookup(local.master_instance_class, "externalIPAddresses", [])
  external_subnet_ids           = lookup(local.master_instance_class, "externalSubnetIDs", [])
  external_subnet_id_deprecated = lookup(local.master_instance_class, "externalSubnetID", null)

  network_type      = contains(keys(local.master_instance_class), "networkType") ? var.network_types[local.master_instance_class.networkType] : null
  additional_labels = merge(lookup(var.providerClusterConfiguration, "labels", {}), lookup(local.master_instance_class, "additionalLabels", {}))
}
