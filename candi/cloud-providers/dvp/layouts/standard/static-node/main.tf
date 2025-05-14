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
}

module "ipv4-address" {
  source       = "../../../terraform-modules/ipv4-address/"
  namespace    = local.namespace
  hostname     = local.hostname
  ipv4_address = local.ipv4_address
}

module "static-node" {
  source                 = "../../../terraform-modules/static-node/"
  prefix                 = local.prefix
  node_group             = local.node_group
  namespace              = local.namespace
  node_index             = local.node_index
  root_disk              = module.root-disk
  ipv4_address           = module.ipv4-address
  memory_size            = local.memory_size
  bootloader             = local.bootloader
  cpu                    = local.cpu
  ssh_public_key         = local.ssh_public_key
  hostname               = local.hostname
  cluster_uuid           = local.cluster_uuid
  additional_labels      = local.additional_labels
  additional_annotations = local.additional_annotations
  priority_class_name    = local.priority_class_name
  node_selector          = local.node_selector
  tolerations            = local.tolerations
  region                 = local.region
  zone                   = local.zone
  cloud_config           = local.user_data
}
