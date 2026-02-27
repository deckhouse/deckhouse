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

module "root-disk" {
  source        = "../../../terraform-modules/root-disk/"
  prefix        = local.prefix
  node_group    = local.node_group
  node_index    = local.node_index
  namespace     = local.namespace
  image         = local.root_disk_image
  size          = local.root_disk_size
  storage_class = local.root_disk_storage_class
  hostname      = local.hostname
  cluster_uuid  = local.cluster_uuid
}

resource "kubernetes_owner_references_v1" "root-disk" {
  api_version = "virtualization.deckhouse.io/v1alpha2"
  kind        = "VirtualDisk"
  metadata {
    name      = module.root-disk.name
    namespace = local.namespace
  }

  owner_references {
    api_version = "virtualization.deckhouse.io/v1alpha2"
    kind        = "VirtualMachine"
    name        = module.static-node.name
    uid         = module.static-node.uid
  }
}

module "ipv4-address" {
  source       = "../../../terraform-modules/ipv4-address/"
  namespace    = local.namespace
  hostname     = local.hostname
  ipv4_address = local.ipv4_address
  cluster_uuid = local.cluster_uuid
}

resource "kubernetes_owner_references_v1" "ipv4-address" {
  api_version = "virtualization.deckhouse.io/v1alpha2"
  kind        = "VirtualMachineIPAddress"
  metadata {
    name      = module.ipv4-address.name
    namespace = local.namespace
  }

  owner_references {
    api_version = "virtualization.deckhouse.io/v1alpha2"
    kind        = "VirtualMachine"
    name        = module.static-node.name
    uid         = module.static-node.uid
  }
}

module "additional-disk" {
  source = "../../../terraform-modules/additional-disk"

  for_each = {
    for i, d in local.additional_disks : tostring(i) => d
  }

  api_version   = "virtualization.deckhouse.io/v1alpha2"
  prefix        = local.prefix
  node_group    = local.node_group
  node_index    = local.node_index
  disk_index    = tonumber(each.key)
  namespace     = local.namespace
  storage_class = try(each.value.storage_class, null)
  size          = each.value.size
  hostname      = local.hostname
  cluster_uuid  = local.cluster_uuid
}

locals {
  static_additional_disks = [
    for k in sort(keys(module.additional-disk)) : {
      name   = module.additional-disk[k].name
      hash   = module.additional-disk[k].hash
      md5_id = module.additional-disk[k].md5_id
    }
  ]
}

resource "kubernetes_owner_references_v1" "additional-disk" {
  for_each = module.additional-disk

  api_version = "virtualization.deckhouse.io/v1alpha2"
  kind        = "VirtualDisk"
  metadata {
    name      = each.value.name
    namespace = local.namespace
  }

  owner_references {
    api_version = "virtualization.deckhouse.io/v1alpha2"
    kind        = "VirtualMachine"
    name        = module.static-node.name
    uid         = module.static-node.uid
  }
}

module "static-node" {
  source                     = "../../../terraform-modules/static-node/"
  prefix                     = local.prefix
  node_group                 = local.node_group
  namespace                  = local.namespace
  node_index                 = local.node_index
  root_disk                  = module.root-disk
  ipv4_address               = module.ipv4-address
  memory_size                = local.memory_size
  virtual_machine_class_name = local.virtual_machine_class_name
  bootloader                 = local.bootloader
  live_migration_policy      = local.live_migration_policy
  run_policy                 = local.run_policy
  cpu                        = local.cpu
  ssh_public_key             = local.ssh_public_key
  hostname                   = local.hostname
  cluster_uuid               = local.cluster_uuid
  additional_labels          = local.additional_labels
  additional_annotations     = local.additional_annotations
  priority_class_name        = local.priority_class_name
  node_selector              = local.node_selector
  tolerations                = local.tolerations
  region                     = local.region
  zone                       = local.zone
  cloud_config               = local.user_data
  additional_disks           = local.static_additional_disks
}

resource "kubernetes_owner_references_v1" "cloudinit-secret" {
  api_version = "v1"
  kind        = "Secret"
  metadata {
    name      = module.static-node.cloudinit_secret_name
    namespace = local.namespace
  }

  owner_references {
    api_version = "virtualization.deckhouse.io/v1alpha2"
    kind        = "VirtualMachine"
    name        = module.static-node.name
    uid         = module.static-node.uid
  }
}
