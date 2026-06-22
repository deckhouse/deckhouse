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
  type    = any
  default = null
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

variable "additional_disks" {
  type = list(object({
    size         = string
    storageClass = optional(string)
  }))
  default = []
}

variable "nodeGroups" {
  type    = any
  default = {}
}

variable "instanceClasses" {
  type    = any
  default = {}
}

variable "secrets" {
  type    = any
  default = {}
}

variable "settings" {
  type    = any
  default = null
}

module "migration" {
  source                       = "../../../terraform-modules/migration"
  providerClusterConfiguration = var.providerClusterConfiguration
  nodeGroups                   = var.nodeGroups
  instanceClasses              = var.instanceClasses
  secrets                      = var.secrets
  settings                     = var.settings
}

locals {
  prefix     = var.clusterConfiguration.cloud.prefix
  node_index = var.nodeIndex

  _master_ng      = lookup(module.migration.nodeGroups, "master", null)
  _master_ic_name = try(local._master_ng.spec.cloudInstances.classReference.name, "")
  instance_class  = try(module.migration.instanceClasses[local._master_ic_name].spec, {})

  namespace      = try(module.migration.settings.spec.settings.provider.parameters.namespace, "")
  ssh_public_key = try(module.migration.settings.spec.settings.nodes.parameters.sshPublicKey, "")
  region         = try(module.migration.settings.spec.settings.nodes.parameters.region, "")
  actual_zones   = try(module.migration.settings.spec.settings.nodes.parameters.zones, [])
  zones          = try(local._master_ng.spec.cloudInstances.zones, null) != null ? tolist(setintersection(local.actual_zones, local._master_ng.spec.cloudInstances.zones)) : local.actual_zones
  zone           = length(local.actual_zones) > 0 ? element(local.zones, var.nodeIndex) : ""

  node_replicas = try(local._master_ng.spec.cloudInstances.minPerZone, 1)

  ipv4_address = try(module.migration.settings.spec.settings.nodes.parameters.ipAddresses["master"], null) == null ? "Auto" : (
    var.nodeIndex + 1 > length(try(module.migration.settings.spec.settings.nodes.parameters.ipAddresses["master"], [])) ? "Auto" :
    try(module.migration.settings.spec.settings.nodes.parameters.ipAddresses["master"], [])[var.nodeIndex]
  )

  cluster_uuid = var.clusterUUID

  root_disk_image = {
    kind = local.instance_class.rootDisk.image.kind
    name = local.instance_class.rootDisk.image.name
  }
  root_disk_size          = local.instance_class.rootDisk.size
  root_disk_storage_class = lookup(local.instance_class.rootDisk, "storageClass", null)

  additional_disks = [
    for d in try(local.instance_class.additionalDisks, []) : {
      size          = d.size
      storage_class = try(d.storageClass, null)
    }
  ]

  cpu = {
    cores         = local.instance_class.virtualMachine.cpu.cores
    core_fraction = lookup(local.instance_class.virtualMachine.cpu, "coreFraction", "100%")
  }
  memory_size                = local.instance_class.virtualMachine.memory.size
  virtual_machine_class_name = local.instance_class.virtualMachine.virtualMachineClassName
  bootloader                 = lookup(local.instance_class.virtualMachine, "bootloader", null)
  live_migration_policy      = lookup(local.instance_class.virtualMachine, "liveMigrationPolicy", "PreferForced")
  run_policy = lookup(
    local.instance_class.virtualMachine,
    "runPolicy",
    "AlwaysOnUnlessStoppedManually",
  )

  kubernetes_data_disk_storage_class = lookup(local.instance_class.etcdDisk, "storageClass", null)
  kubernetes_data_disk_size          = local.instance_class.etcdDisk.size

  additional_labels      = lookup(local.instance_class.virtualMachine, "additionalLabels", {})
  additional_annotations = lookup(local.instance_class.virtualMachine, "additionalAnnotations", {})
  priority_class_name    = lookup(local.instance_class.virtualMachine, "priorityClassName", null)
  node_selector          = lookup(local.instance_class.virtualMachine, "nodeSelector", {})
  tolerations            = lookup(local.instance_class.virtualMachine, "tolerations", null)

  node_group = "master"
  hostname   = join("-", [local.prefix, local.node_group, local.node_index])
  user_data  = var.cloudConfig == "" ? "" : var.cloudConfig
}
