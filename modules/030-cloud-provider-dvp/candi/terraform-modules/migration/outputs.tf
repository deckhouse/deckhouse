# Copyright 2026 Flant JSC
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

output "settings" {
  description = "Resolved ModuleConfig object for cloud-provider-dvp."
  value       = jsondecode(local.use_pcc ? jsonencode(local._pcc_module_config) : jsonencode(var.settings))
}

output "nodeGroups" {
  description = "Map of resolved NodeGroup objects keyed by node group name."
  value       = jsondecode(local.use_pcc ? jsonencode(local._pcc_node_groups) : jsonencode(var.nodeGroups))
}

output "instanceClasses" {
  description = "Map of resolved DVPInstanceClass objects keyed by instance class name."
  value       = jsondecode(local.use_pcc ? jsonencode(local._pcc_instance_classes) : jsonencode(var.instanceClasses))
}

output "secrets" {
  description = "Map of resolved credential Secret objects keyed by secret name."
  sensitive   = true
  value       = jsondecode(local.use_pcc ? jsonencode(local._pcc_credential_secrets) : jsonencode(var.secrets))
}
