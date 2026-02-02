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
# It uses null_resource with local-exec to validate resources in parent DVP cluster.
# Will fail fast with clear error messages if VirtualMachineClass or images are not found.

# Validate VirtualMachineClass exists using kubectl
resource "null_resource" "validate_vm_class" {
  provisioner "local-exec" {
    command = <<-EOT
      if ! kubectl get virtualmachineclass ${var.virtual_machine_class_name} >/dev/null 2>&1; then
        echo "ERROR: VirtualMachineClass '${var.virtual_machine_class_name}' not found in parent DVP cluster."
        echo "Please ensure the VirtualMachineClass exists before creating VMs."
        echo "Available VirtualMachineClasses can be listed with: kubectl get virtualmachineclasses"
        exit 1
      fi
      echo "✓ VirtualMachineClass '${var.virtual_machine_class_name}' validated successfully"
    EOT
  }
}

# Validate ClusterVirtualImage exists (if specified)
resource "null_resource" "validate_cluster_image" {
  count = var.image_kind == "ClusterVirtualImage" ? 1 : 0

  provisioner "local-exec" {
    command = <<-EOT
      if ! kubectl get clustervirtualimage ${var.image_name} >/dev/null 2>&1; then
        echo "ERROR: ClusterVirtualImage '${var.image_name}' not found in parent DVP cluster."
        echo "Please ensure the image exists before creating VMs."
        echo "Available ClusterVirtualImages can be listed with: kubectl get clustervirtualimages"
        exit 1
      fi
      echo "✓ ClusterVirtualImage '${var.image_name}' validated successfully"
    EOT
  }

  depends_on = [null_resource.validate_vm_class]
}

# Validate VirtualImage exists in namespace (if specified)
resource "null_resource" "validate_virtual_image" {
  count = var.image_kind == "VirtualImage" ? 1 : 0

  provisioner "local-exec" {
    command = <<-EOT
      if ! kubectl get virtualimage ${var.image_name} -n ${var.namespace} >/dev/null 2>&1; then
        echo "ERROR: VirtualImage '${var.image_name}' not found in namespace '${var.namespace}' in parent DVP cluster."
        echo "Please ensure the image exists before creating VMs."
        echo "Available VirtualImages can be listed with: kubectl get virtualimages -n ${var.namespace}"
        exit 1
      fi
      echo "✓ VirtualImage '${var.image_name}' validated successfully in namespace '${var.namespace}'"
    EOT
  }

  depends_on = [null_resource.validate_vm_class]
}

# Output validation results for logging
output "validation_status" {
  value = {
    virtual_machine_class_name = var.virtual_machine_class_name
    image_kind                 = var.image_kind
    image_name                 = var.image_name
    namespace                  = var.namespace
    validation_completed       = true
  }
  description = "Validation status of DVP resources"

  depends_on = [
    null_resource.validate_vm_class,
    null_resource.validate_cluster_image,
    null_resource.validate_virtual_image
  ]
}
