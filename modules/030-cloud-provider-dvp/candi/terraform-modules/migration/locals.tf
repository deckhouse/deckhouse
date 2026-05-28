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

locals {
  # --- Source-of-truth detection ---

  has_pcc = var.providerClusterConfiguration != null

  has_node_groups       = var.nodeGroups != null && length(var.nodeGroups) > 0
  has_instance_classes  = var.instanceClasses != null && length(var.instanceClasses) > 0
  has_credential_secret = var.secrets != null && length(var.secrets) > 0

  # New resources are considered complete only when all three parts are present.
  new_resources_complete = local.has_node_groups && local.has_instance_classes && local.has_credential_secret

  # Use PCC when it is present and new resources are not yet complete (migration in progress).
  use_pcc = local.has_pcc && !local.new_resources_complete

  # --- PCC-derived values ---

  pcc_kubeconfig     = try(var.providerClusterConfiguration.provider.kubeconfigDataBase64, "")
  pcc_namespace      = try(var.providerClusterConfiguration.provider.namespace, "")
  pcc_network_policy = try(var.providerClusterConfiguration.provider.networkPolicy, "Isolated")
  pcc_ssh_public_key = try(var.providerClusterConfiguration.sshPublicKey, "")
  pcc_region         = try(var.providerClusterConfiguration.region, "")
  pcc_zones          = try(var.providerClusterConfiguration.zones, [])

  pcc_master_node_group = {
    replicas      = try(var.providerClusterConfiguration.masterNodeGroup.replicas, 1)
    zones         = try(var.providerClusterConfiguration.masterNodeGroup.zones, null)
    instanceClass = try(var.providerClusterConfiguration.masterNodeGroup.instanceClass, {})
  }

  pcc_node_groups = try(var.providerClusterConfiguration.nodeGroups, [])

  # --- New-resources-derived values ---

  # First Secret of type "cloud-provider.deckhouse.io/credentials" provides the kubeconfig.
  new_kubeconfig = try(
    [
      for name, s in var.secrets : s.stringData.secret
      if try(s.stringData.secret, null) != null && try(s.type, "") == "cloud-provider.deckhouse.io/credentials"
    ][0],
    ""
  )

  new_namespace      = try(var.settings.spec.settings.provider.parameters.namespace, "")
  new_network_policy = try(var.settings.spec.settings.provider.parameters.networkPolicy, "Isolated")
  new_ssh_public_key = try(var.settings.spec.settings.nodes.parameters.sshPublicKey, "")
  new_region         = try(var.settings.spec.settings.nodes.parameters.region, "")
  new_zones          = try(var.settings.spec.settings.nodes.parameters.zones, [])

  new_master_node_group = {
    replicas = try(var.nodeGroups["master"].spec.cloudInstances.minPerZone, 1)
    zones    = try(var.nodeGroups["master"].spec.cloudInstances.zones, null)
    instanceClass = try(
      var.instanceClasses[try(var.nodeGroups["master"].spec.cloudInstances.classReference.name, "")].spec,
      {}
    )
  }

  new_node_groups = [
    for name, ng in var.nodeGroups : {
      name     = name
      replicas = try(ng.spec.cloudInstances.minPerZone, 1)
      zones    = try(ng.spec.cloudInstances.zones, null)
      instanceClass = try(
        var.instanceClasses[try(ng.spec.cloudInstances.classReference.name, "")].spec,
        {}
      )
    }
    if name != "master"
  ]

  # --- Unified outputs ---

  kubeconfig_base64 = local.use_pcc ? local.pcc_kubeconfig : local.new_kubeconfig
  namespace         = local.use_pcc ? local.pcc_namespace : local.new_namespace
  network_policy    = local.use_pcc ? local.pcc_network_policy : local.new_network_policy
  ssh_public_key    = local.use_pcc ? local.pcc_ssh_public_key : local.new_ssh_public_key
  region            = local.use_pcc ? local.pcc_region : local.new_region
  zones             = local.use_pcc ? local.pcc_zones : local.new_zones
  master_node_group = local.use_pcc ? local.pcc_master_node_group : local.new_master_node_group
  node_groups       = local.use_pcc ? local.pcc_node_groups : local.new_node_groups
}
