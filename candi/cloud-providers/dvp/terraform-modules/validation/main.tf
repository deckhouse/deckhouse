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

# This module validates that required DVP resources exist before attempting to create VMs.
# Uses kubernetes_resource data sources to query parent DVP cluster.
# Will fail with clear error messages if VirtualMachineClass or images are not found.

# Validate VirtualMachineClass exists
data "kubernetes_resource" "virtual_machine_class" {
  api_version = var.api_version
  kind        = "VirtualMachineClass"

  metadata {
    name = var.virtual_machine_class_name
  }
}

# Validate image exists based on kind
data "kubernetes_resource" "image" {
  api_version = var.api_version
  kind        = var.image_kind

  metadata {
    name      = var.image_name
    namespace = var.image_kind == "VirtualImage" ? var.namespace : null
  }
}

# Output validation results for logging
output "validation_status" {
  value = {
    virtual_machine_class_name = var.virtual_machine_class_name
    virtual_machine_class_uid  = data.kubernetes_resource.virtual_machine_class.object.metadata.uid
    image_kind                 = var.image_kind
    image_name                 = var.image_name
    image_uid                  = data.kubernetes_resource.image.object.metadata.uid
    namespace                  = var.namespace
    validation_completed       = "true"
  }
  description = "Validation status of DVP resources"
}
