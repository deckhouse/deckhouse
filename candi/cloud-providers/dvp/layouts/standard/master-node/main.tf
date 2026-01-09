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

module "master" {
  source                     = "../../../terraform-modules/master"
  prefix                     = local.prefix
  node_group                 = local.node_group
  namespace                  = local.namespace
  node_index                 = local.node_index
  root_disk_name             = local.root_disk_name
  kubernetes_data_disk_name  = local.data_disk_name
  ip_address_name            = local.ip_address_name
  memory_size                = local.memory_size
  virtual_machine_class_name = local.virtual_machine_class_name
  bootloader                 = local.bootloader
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
  additional_disks           = local.master_additional_disks
}

module "ipv4-address" {
  source          = "../../../terraform-modules/ipv4-address/"
  namespace       = local.namespace
  ipv4_address    = local.ipv4_address
  ip_address_name = local.ip_address_name
  owner_ref_name  = module.master.vm_name
  owner_ref_uid   = module.master.uid
}

module "root-disk" {
  source                                 = "../../../terraform-modules/root-disk/"
  root_disk_destructive_params_json      = local.root_disk_destructive_params_json
  root_disk_destructive_params_json_hash = local.root_disk_destructive_params_json_hash
  root_disk_name                         = local.root_disk_name
  namespace                              = local.namespace
  image                                  = local.root_disk_image
  size                                   = local.root_disk_size
  storage_class                          = local.root_disk_storage_class
  owner_ref_name                         = module.master.vm_name
  owner_ref_uid                          = module.master.uid
}

module "kubernetes-data-disk" {
  source                                 = "../../../terraform-modules/kubernetes-data-disk/"
  data_disk_destructive_params_json      = local.root_disk_destructive_params_json
  data_disk_destructive_params_json_hash = local.root_disk_destructive_params_json_hash
  data_disk_name                         = local.data_disk_name
  namespace                              = local.namespace
  storage_class                          = local.kubernetes_data_disk_storage_class
  size                                   = local.kubernetes_data_disk_size
  owner_ref_name                         = module.master.vm_name
  owner_ref_uid                          = module.master.uid
}

module "additional-disk" {
  source = "../../../terraform-modules/additional-disk"

  for_each = {
    for i, d in local.additional_disks : tostring(i) => d
  }

  api_version                       = "virtualization.deckhouse.io/v1alpha2"
  disk_destructive_params_json      = each.value.disk_destructive_params_json
  disk_destructive_params_json_hash = each.value.disk_destructive_params_json_hash
  disk_name                         = each.value.disk_name
  namespace                         = local.namespace
  storage_class                     = try(each.value.storage_class, null)
  size                              = each.value.size
  owner_ref_name                    = module.master.vm_name
  owner_ref_uid                     = module.master.uid
}
