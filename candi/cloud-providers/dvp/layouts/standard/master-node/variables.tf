# Copyright 2025 Flant JSC
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


locals {
  prefix            = var.clusterConfiguration.cloud.prefix
  node_index        = var.nodeIndex
  namespace         = var.providerClusterConfiguration.provider.namespace
  master_node_group = var.providerClusterConfiguration.masterNodeGroup
  instance_class    = local.master_node_group.instanceClass


  cluster_uuid = var.clusterUUID

  root_disk_image = {
    kind = local.instance_class.rootDisk.image.kind
    name = local.instance_class.rootDisk.image.name
  }
  root_disk_size          = local.instance_class.rootDisk.size
  root_disk_storage_class = lookup(local.instance_class.rootDisk, "storageClass", null)

  cpu = {
    cores         = local.instance_class.virtualMachine.cpu.cores
    core_fraction = lookup(local.instance_class.virtualMachine.cpu, "coreFraction", "100%")
  }
  memory_size = local.instance_class.virtualMachine.memory.size
  bootloader  = lookup(local.instance_class.virtualMachine, "bootloader", null)

  ssh_public_key = var.providerClusterConfiguration.sshPublicKey

  ipv4_address = lookup(local.instance_class.virtualMachine, "ipAddresses", null) == null ? "Auto" : local.node_index + 1 > length(local.instance_class.virtualMachine.ipAddresses) ? "Auto" : local.instance_class.virtualMachine.ipAddresses[local.node_index]

  kubernetes_data_disk_storage_class = lookup(local.instance_class.etcdDisk, "storageClass", null)
  kubernetes_data_disk_size          = local.instance_class.etcdDisk.size

  region = lookup(var.providerClusterConfiguration, "region", "")

  actual_zones = lookup(var.providerClusterConfiguration, "zones", [])
  zones        = lookup(local.master_node_group, "zones", null) != null ? tolist(setintersection(local.actual_zones, local.master_node_group["zones"])) : local.actual_zones
  zone         = length(local.actual_zones) > 0 ? element(local.zones, var.nodeIndex) : ""

  additional_labels      = lookup(local.instance_class.virtualMachine, "additionalLabels", {})
  additional_annotations = lookup(local.instance_class.virtualMachine, "additionalAnnotations", {})
  priority_class_name    = lookup(local.instance_class.virtualMachine, "priorityClassName", null)
  node_selector          = lookup(local.instance_class.virtualMachine, "nodeSelector", {})
  tolerations            = lookup(local.instance_class.virtualMachine, "tolerations", null)

  node_group = "master"
  hostname   = join("-", [local.prefix, local.node_group, local.node_index])
  user_data  = var.cloudConfig == "" ? "" : var.cloudConfig
}

