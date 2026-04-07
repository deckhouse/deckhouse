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

variable "api_version" {
  type        = string
  default     = "virtualization.deckhouse.io/v1alpha2"
  description = "API version for DVP virtualization resources"
}

variable "namespace" {
  type        = string
  description = "Namespace in parent DVP cluster where VirtualImage resources should be validated"
}

variable "virtual_machine_class_name" {
  type        = string
  description = "Name of the VirtualMachineClass to validate in parent DVP cluster"
}

variable "image_kind" {
  type        = string
  description = "Kind of the boot disk image (ClusterVirtualImage or VirtualImage)"

  validation {
    condition     = contains(["ClusterVirtualImage", "VirtualImage"], var.image_kind)
    error_message = "image_kind must be either 'ClusterVirtualImage' or 'VirtualImage'"
  }
}

variable "image_name" {
  type        = string
  description = "Name of the boot disk image to validate in parent DVP cluster"
}
