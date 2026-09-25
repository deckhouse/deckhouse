# Copyright 2026 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "settings" {
  description = "Resolved ModuleConfig object for cloud-provider-zvirt."
  value       = local.resolved_settings

  precondition {
    condition     = !local._configured || local.has_pcc || local._mc_version >= 2
    error_message = <<-EOT
      ERROR: no cloud-provider-zvirt configuration source found.

      Supply either a ZvirtClusterConfiguration (legacy flow) or a ModuleConfig
      `cloud-provider-zvirt` with `spec.version: 2`.
    EOT
  }

  precondition {
    condition     = !local._configured || contains(local._known_layouts, local._layout)
    error_message = "ERROR: unknown layout '${local._layout}': expected one of ${join(", ", local._known_layouts)}."
  }

  precondition {
    condition     = !local._configured || local._server != ""
    error_message = <<-EOT
      ERROR: the zVirt API endpoint is not set.

      Set `spec.settings.provider.parameters.server` in the cloud-provider-zvirt
      ModuleConfig (or `provider.server` in ZvirtClusterConfiguration).
    EOT
  }

  precondition {
    condition     = !local._configured || local._cluster_id != ""
    error_message = <<-EOT
      ERROR: the zVirt cluster ID is not set.

      Set `spec.settings.provider.parameters.clusterID` in the cloud-provider-zvirt
      ModuleConfig (or `clusterID` in ZvirtClusterConfiguration).
    EOT
  }

  precondition {
    condition     = length(local._dangling_class_references) == 0
    error_message = <<-EOT
      ERROR: NodeGroups ${join(", ", local._dangling_class_references)} reference a
      ZvirtInstanceClass that does not exist.

      Available instance classes: ${join(", ", keys(local.resolved_instance_classes))}.
    EOT
  }

  precondition {
    condition     = length(local._wrong_kind_class_references) == 0
    error_message = "ERROR: NodeGroups ${join(", ", local._wrong_kind_class_references)} do not reference a ZvirtInstanceClass in spec.cloudInstances.classReference.kind."
  }

  precondition {
    condition     = local._empty_pcc_ng_names == 0
    error_message = "ERROR: ZvirtClusterConfiguration has a nodeGroups entry with an empty name."
  }

  precondition {
    condition     = !local._duplicate_pcc_ng_names
    error_message = "ERROR: ZvirtClusterConfiguration nodeGroups names must be unique, got: ${join(", ", local._pcc_ng_names)}."
  }

  precondition {
    condition     = !local._configured || try(local._resolved_credentials[local._credentials_name].identity, "") != ""
    error_message = <<-EOT
      ERROR: no zVirt credentials available.

      Expected a Secret `d8-credentials` of type cloud-provider.deckhouse.io/credentials
      in the d8-cloud-provider-zvirt namespace (or `provider.username` and
      `provider.password` in ZvirtClusterConfiguration).
    EOT
  }
}

output "nodeGroups" {
  description = "Map of resolved NodeGroup objects keyed by node group name."
  value       = local.resolved_node_groups
}

output "instanceClasses" {
  description = "Map of resolved ZvirtInstanceClass objects keyed by instance class name."
  value       = local.resolved_instance_classes
}

output "credentials" {
  description = "Resolved credential Secret values keyed by Secret name."
  sensitive   = true
  value       = local._resolved_credentials
}

output "secrets" {
  description = "Map of resolved credential Secret objects keyed by secret name."
  sensitive   = true
  value       = local.resolved_secrets
}
