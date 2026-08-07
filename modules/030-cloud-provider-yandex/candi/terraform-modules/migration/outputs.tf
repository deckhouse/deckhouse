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
  description = "Resolved ModuleConfig object for cloud-provider-yandex."
  value       = jsondecode(local.use_pcc ? jsonencode(local._pcc_module_config) : jsonencode(var.settings))

  # See validation.tf for why the configuration checks live here.
  precondition {
    condition     = !local._configured || local.has_pcc || local._mc_version >= 2
    error_message = <<-EOT
      ERROR: no cloud-provider-yandex configuration source found.

      Supply either a YandexClusterConfiguration (legacy flow) or a
      ModuleConfig `cloud-provider-yandex` with `spec.version: 2`.
    EOT
  }

  precondition {
    condition     = !local._configured || local._network_cidr != ""
    error_message = <<-EOT
      ERROR: nodeNetworkCIDR is not set.

      Set `spec.settings.nodes.parameters.nodeNetworkCIDR` in the
      cloud-provider-yandex ModuleConfig (or `nodeNetworkCIDR` in
      YandexClusterConfiguration).
    EOT
  }

  precondition {
    condition = (
      !local._configured || local._network_cidr == "" ||
      (can(cidrsubnet(local._network_cidr, 0, 0)) && cidrsubnet(local._network_cidr, 0, 0) == local._network_cidr)
    )
    error_message = "ERROR: invalid nodeNetworkCIDR '${local._network_cidr}': expected a network address in CIDR notation, for example 10.241.0.0/16."
  }

  precondition {
    condition     = !local._configured || contains(local._known_layouts, local._layout)
    error_message = "ERROR: unknown layout '${local._layout}': expected one of ${join(", ", local._known_layouts)}."
  }

  precondition {
    condition     = length(local._dangling_class_references) == 0
    error_message = <<-EOT
      ERROR: NodeGroups ${join(", ", local._dangling_class_references)} reference a
      YandexInstanceClass that does not exist.

      Available instance classes: ${join(", ", keys(local.resolved_instance_classes))}.
    EOT
  }

  precondition {
    condition     = length(local._wrong_kind_class_references) == 0
    error_message = "ERROR: NodeGroups ${join(", ", local._wrong_kind_class_references)} do not reference a YandexInstanceClass in spec.cloudInstances.classReference.kind."
  }

  precondition {
    condition     = local._empty_pcc_ng_names == 0
    error_message = "ERROR: YandexClusterConfiguration has a nodeGroups entry with an empty name."
  }

  precondition {
    condition     = !local._duplicate_pcc_ng_names
    error_message = "ERROR: YandexClusterConfiguration nodeGroups names must be unique, got: ${join(", ", local._pcc_ng_names)}."
  }

  precondition {
    condition     = !local._configured || try(local._resolved_credentials[local._credentials_name], "") != ""
    error_message = <<-EOT
      ERROR: no Yandex Cloud service account key available.

      Expected a Secret `d8-credentials` of type cloud-provider.deckhouse.io/credentials
      in the d8-cloud-provider-yandex namespace (or `provider.serviceAccountJSON`
      in YandexClusterConfiguration).
    EOT
  }

}

output "nodeGroups" {
  description = "Map of resolved NodeGroup objects keyed by node group name."
  value       = jsondecode(local.use_pcc ? jsonencode(local._pcc_node_groups) : jsonencode(var.nodeGroups))
}

output "instanceClasses" {
  description = "Map of resolved YandexInstanceClass objects keyed by instance class name."
  value       = jsondecode(local.use_pcc ? jsonencode(local._pcc_instance_classes) : jsonencode(var.instanceClasses))
}

# The module already resolves which source of truth wins, so the credential lookup belongs
# here rather than being repeated by every consumer: providers.tf configures the Yandex
# provider from it and the WithNATInstance layout takes the exporter key from it.
# Sensitive because these are service-account keys and API tokens; consumers that publish
# a value derived from them have to unwrap it explicitly.
output "credentials" {
  description = "Resolved credential Secret values keyed by Secret name."
  sensitive   = true
  value       = local._resolved_credentials
}

output "secrets" {
  description = "Map of resolved credential Secret objects keyed by secret name."
  sensitive   = true
  value       = jsondecode(local.use_pcc ? jsonencode(local._pcc_credential_secrets) : jsonencode(var.secrets))
}
