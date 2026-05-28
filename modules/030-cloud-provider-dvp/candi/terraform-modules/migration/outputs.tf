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

output "kubeconfig_base64" {
  description = "Base64-encoded kubeconfig for connecting to the parent DVP cluster."
  value       = local.kubeconfig_base64
  sensitive   = true
}

output "namespace" {
  description = "Namespace in the parent DVP cluster where VM resources are managed."
  value       = local.namespace
}

output "network_policy" {
  description = "Network policy mode for the parent DVP cluster (e.g. Isolated)."
  value       = local.network_policy
}

output "ssh_public_key" {
  description = "SSH public key injected into provisioned nodes."
  value       = local.ssh_public_key
  sensitive   = true
}

output "region" {
  description = "Region label used for zone-aware scheduling."
  value       = local.region
}

output "zones" {
  description = "List of availability zones available for node placement."
  value       = local.zones
}

output "master_node_group" {
  description = "Resolved master NodeGroup definition compatible with PCC.masterNodeGroup shape."
  value       = local.master_node_group
}

output "node_groups" {
  description = "List of resolved worker NodeGroup definitions compatible with PCC.nodeGroups shape."
  value       = local.node_groups
}
