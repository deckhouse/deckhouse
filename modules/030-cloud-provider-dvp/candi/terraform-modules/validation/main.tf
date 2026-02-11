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
# Uses kubernetes_resources (plural) data source to list and validate resources.
# Will fail with clear error messages if VirtualMachineClass or images are not found.

# List all VirtualMachineClasses to validate the specified one exists
data "kubernetes_resources" "virtual_machine_classes" {
  api_version = var.api_version
  kind        = "VirtualMachineClass"
}

# List images based on kind
data "kubernetes_resources" "images" {
  api_version = var.api_version
  kind        = var.image_kind
  namespace   = var.image_kind == "VirtualImage" ? var.namespace : null
}

# Dummy resource to validate and fail with clear error messages
resource "terraform_data" "validation" {
  lifecycle {
    precondition {
      condition = contains(
        [for vmc in data.kubernetes_resources.virtual_machine_classes.objects : vmc.metadata.name],
        var.virtual_machine_class_name
      )
      error_message = <<-EOT
        ERROR: VirtualMachineClass '${var.virtual_machine_class_name}' not found in parent DVP cluster.

        Please ensure the VirtualMachineClass exists before creating VMs.

        Available VirtualMachineClasses:
        ${join("\n", [for vmc in data.kubernetes_resources.virtual_machine_classes.objects : "  - ${vmc.metadata.name}"])}
      EOT
    }

    precondition {
      condition = contains(
        [for img in data.kubernetes_resources.images.objects : img.metadata.name],
        var.image_name
      )
      error_message = <<-EOT
        ERROR: ${var.image_kind} '${var.image_name}' not found${var.image_kind == "VirtualImage" ? " in namespace '${var.namespace}'" : ""} in parent DVP cluster.

        Please ensure the image exists before creating VMs.

        Available ${var.image_kind}s:
        ${join("\n", [for img in data.kubernetes_resources.images.objects : "  - ${img.metadata.name}"])}
      EOT
    }
  }
}

# Output validation results for logging
output "validation_status" {
  value = {
    virtual_machine_class_name = var.virtual_machine_class_name
    image_kind                 = var.image_kind
    image_name                 = var.image_name
    namespace                  = var.namespace
    validation_completed       = "true"
    available_vm_classes       = [for vmc in data.kubernetes_resources.virtual_machine_classes.objects : vmc.metadata.name]
    available_images           = [for img in data.kubernetes_resources.images.objects : img.metadata.name]
  }
  description = "Validation status of DVP resources"
  depends_on  = [terraform_data.validation]
}
