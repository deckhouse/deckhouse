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
# It uses data sources to query the parent DVP cluster and will fail fast with clear error messages
# if VirtualMachineClass or images are not found.

# Validate VirtualMachineClass exists
data "kubernetes_resource" "virtual_machine_class" {
  api_version = var.api_version
  kind        = "VirtualMachineClass"

  metadata {
    name = var.virtual_machine_class_name
  }

  lifecycle {
    postcondition {
      condition     = self.object != null
      error_message = "VirtualMachineClass '${var.virtual_machine_class_name}' not found in parent DVP cluster. Please ensure the VirtualMachineClass exists before creating VMs. Available VirtualMachineClasses can be listed with: kubectl get virtualmachineclasses"
    }
  }
}

# Validate ClusterVirtualImage exists (if specified)
data "kubernetes_resource" "cluster_virtual_image" {
  count = var.image_kind == "ClusterVirtualImage" ? 1 : 0

  api_version = var.api_version
  kind        = "ClusterVirtualImage"

  metadata {
    name = var.image_name
  }

  lifecycle {
    postcondition {
      condition     = self.object != null
      error_message = "ClusterVirtualImage '${var.image_name}' not found in parent DVP cluster. Please ensure the image exists before creating VMs. Available ClusterVirtualImages can be listed with: kubectl get clustervirtualimages"
    }
  }
}

# Validate VirtualImage exists in namespace (if specified)
data "kubernetes_resource" "virtual_image" {
  count = var.image_kind == "VirtualImage" ? 1 : 0

  api_version = var.api_version
  kind        = "VirtualImage"

  metadata {
    name      = var.image_name
    namespace = var.namespace
  }

  lifecycle {
    postcondition {
      condition     = self.object != null
      error_message = "VirtualImage '${var.image_name}' not found in namespace '${var.namespace}' in parent DVP cluster. Please ensure the image exists before creating VMs. Available VirtualImages can be listed with: kubectl get virtualimages -n ${var.namespace}"
    }
  }
}

# Output validation results for logging
output "validation_status" {
  value = {
    virtual_machine_class_validated = data.kubernetes_resource.virtual_machine_class.object.metadata.name
    image_kind                      = var.image_kind
    image_name                      = var.image_name
    image_validated                 = true
  }
  description = "Validation status of DVP resources"
}
