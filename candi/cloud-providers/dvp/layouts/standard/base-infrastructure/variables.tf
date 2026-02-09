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

# Extract locals needed for validation module
locals {
  namespace         = var.providerClusterConfiguration.provider.namespace
  master_node_group = var.providerClusterConfiguration.masterNodeGroup
  instance_class    = local.master_node_group.instanceClass

  root_disk_image = {
    kind = local.instance_class.rootDisk.image.kind
    name = local.instance_class.rootDisk.image.name
  }

  virtual_machine_class_name = local.instance_class.virtualMachine.virtualMachineClassName
}
