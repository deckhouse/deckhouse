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
# Uses kubernetes_resources with metadata.name field_selector so the apiserver returns at
# most one object — avoids listing every VirtualMachineClass / ClusterVirtualImage in the
# parent cluster (which can be huge in shared environments and adds 10s+ to plan time per
# data source).

# Single VirtualMachineClass by name.
data "kubernetes_resources" "virtual_machine_classes" {
  api_version    = var.api_version
  kind           = "VirtualMachineClass"
  field_selector = "metadata.name=${var.virtual_machine_class_name}"
}

# Single image (ClusterVirtualImage or VirtualImage) by name.
data "kubernetes_resources" "images" {
  api_version    = var.api_version
  kind           = var.image_kind
  namespace      = var.image_kind == "VirtualImage" ? var.namespace : null
  field_selector = "metadata.name=${var.image_name}"
}

# Dummy resource to validate and fail with clear error messages.
resource "terraform_data" "validation" {
  lifecycle {
    precondition {
      condition = length(data.kubernetes_resources.virtual_machine_classes.objects) > 0
      error_message = <<-EOT
        ERROR: VirtualMachineClass '${var.virtual_machine_class_name}' not found in parent DVP cluster.

        Run `kubectl get virtualmachineclasses` in the parent cluster to see available classes
        and update DVPInstanceClass / DVPClusterConfiguration accordingly.
      EOT
    }

    precondition {
      condition = length(data.kubernetes_resources.images.objects) > 0
      error_message = <<-EOT
        ERROR: ${var.image_kind} '${var.image_name}' not found${var.image_kind == "VirtualImage" ? " in namespace '${var.namespace}'" : ""} in parent DVP cluster.

        Run `kubectl get ${lower(var.image_kind)}s${var.image_kind == "VirtualImage" ? " -n ${var.namespace}" : ""}`
        in the parent cluster to see available images and update the config accordingly.
      EOT
    }
  }
}

# Output validation results for logging.
output "validation_status" {
  value = {
    virtual_machine_class_name = var.virtual_machine_class_name
    image_kind                 = var.image_kind
    image_name                 = var.image_name
    namespace                  = var.namespace
    validation_completed       = "true"
  }
  description = "Validation status of DVP resources"
  depends_on  = [terraform_data.validation]
}
