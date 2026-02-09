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

# Validate that required DVP resources exist before proceeding with cluster bootstrap.
# This module will fail fast with clear error messages if VirtualMachineClass or boot images
# are not found in the parent DVP cluster, preventing VM creation from getting stuck in Pending state.
module "validation" {
  source = "../../../terraform-modules/validation"

  namespace                  = local.namespace
  virtual_machine_class_name = local.virtual_machine_class_name
  image_kind                 = local.root_disk_image.kind
  image_name                 = local.root_disk_image.name
}
