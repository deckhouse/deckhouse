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

# This module resolves a single source of truth for the terraform layouts.
#
# Two cluster states are supported:
#
#   State A (migration pending): YandexClusterConfiguration is still the source
#     of truth. The PCC is projected onto the new resource shapes (ModuleConfig
#     v2, NodeGroup, YandexInstanceClass, credential Secret) so every consumer
#     reads the new shapes only.
#   State B (migration done): the cluster resources are the source of truth and
#     are passed through unchanged.
#
# The projection must produce exactly what hooks/internal/migration.go writes
# into the cluster, otherwise terraform sees drift right after the in-cluster
# migration completes and recreates nodes.

locals {
  # --- Constants -------------------------------------------------------------

  _credentials_secret_type   = "cloud-provider.deckhouse.io/credentials"
  _namespace                 = "d8-cloud-provider-yandex"
  _credentials_name          = "d8-credentials"
  _exporter_credentials_name = "d8-credentials-exporter"

  # PCC omits these, but the YandexInstanceClass CRD defaults them on write.
  # The values below are the ones the *previous* terraform code used, so a
  # cluster migrated from a PCC that left them unset keeps its current
  # infrastructure instead of having disks and platforms replaced.
  _default_platform_id  = "standard-v2"
  _default_disk_type    = "network-ssd"
  _default_disk_size_gb = 50
  # openapi/values.yaml defaults masterNodeGroup.instanceClass.etcdDiskSizeGb
  # to 10; candi's cluster_configuration.yaml has no schema default.
  _default_etcd_disk_size_gb = 10

  # --- PCC shorthand ---------------------------------------------------------

  _pcc = var.providerClusterConfiguration

  # An empty object counts as "no PCC". dhctl omits the key entirely on a
  # ModuleConfig-only cluster, but any caller that passes `{}` instead would
  # otherwise enable the PCC branch: every PCC-derived value would be empty, so
  # a perfectly valid ModuleConfig-only cluster would fail on the layout,
  # nodeNetworkCIDR and credentials preconditions.
  has_pcc = var.providerClusterConfiguration != null && length(try(keys(var.providerClusterConfiguration), [])) > 0

  # Explicit nulls are as common as missing keys in a PCC secret, and try() only
  # guards against the latter: tolist(null) raises, so try() catches both.
  _pcc_zones = try(tolist(local._pcc.zones), [])

  # nodeGroups entries are heterogeneous objects (some carry zones, some carry a
  # nodeTemplate), so the tuple cannot be unified with an empty one. Iterating by
  # index sidesteps that: range(0) is empty when the key is missing or null.
  _pcc_node_group_count = try(length(local._pcc.nodeGroups), 0)

  # --- Node group inventory --------------------------------------------------

  # Unified list of all PCC node groups: master (only when it has replicas, so
  # hybrid clusters are not forced to have one) plus the static node groups.
  # replicas needs no default on either branch: the PCC schema requires
  # masterNodeGroup.replicas (minimum 1) and nodeGroups[].replicas, and the branch below is
  # only evaluated once the key is known to exist. A PCC that omits it is invalid, and failing
  # is better than inventing a node count.
  _pcc_all_ngs_list = concat(
    try(local._pcc.masterNodeGroup.replicas, 0) > 0 ? [
      {
        name          = "master"
        replicas      = local._pcc.masterNodeGroup.replicas
        zones         = try(tolist(local._pcc.masterNodeGroup.zones), [])
        instanceClass = try(local._pcc.masterNodeGroup.instanceClass, {})
        nodeTemplate = {
          labels = {
            "node-role.kubernetes.io/control-plane" = ""
            "node-role.kubernetes.io/master"        = ""
          }
        }
      }
    ] : [],
    [
      for i in range(local._pcc_node_group_count) : {
        name          = try(local._pcc.nodeGroups[i].name, "")
        replicas      = local._pcc.nodeGroups[i].replicas
        zones         = try(tolist(local._pcc.nodeGroups[i].zones), [])
        instanceClass = try(local._pcc.nodeGroups[i].instanceClass, {})
        nodeTemplate  = try(local._pcc.nodeGroups[i].nodeTemplate, null)
      }
    ]
  )

  _pcc_ng_names = [for ng in local._pcc_all_ngs_list : ng.name]

  # Group by name before keying maps on it. A duplicated nodeGroups[].name would
  # otherwise abort evaluation with "Duplicate object key" and hide the
  # precondition that reports the actual configuration mistake.
  _pcc_ngs_by_name = { for ng in local._pcc_all_ngs_list : ng.name => ng... }
  _pcc_ngs         = [for name, group in local._pcc_ngs_by_name : group[0]]

  # Mirrors go_lib/cloud-provider/api.BuildInstanceClassName: a readable prefix
  # capped at 50 characters with trailing dashes trimmed, plus a 48-bit SHA-256
  # suffix, keeping the name inside the 63-character DNS-1123 label limit.
  _instance_class_names = {
    for name in keys(local._pcc_ngs_by_name) : name => format(
      "%s-%s",
      replace(substr(name, 0, 50), "/-+$/", ""),
      substr(sha256(name), 0, 12),
    )
  }

  # --- Source-of-truth detection --------------------------------------------

  _mc_version = try(var.settings.spec.version, 0)

  # The same three ModuleConfig conditions IsMigrationResourcesApplied() checks in
  # hooks/internal/migration.go (Version < 2 || !Enabled || len(SettingsV2) == 0).
  # A disabled or settings-less ModuleConfig is not a usable source of truth, and
  # if terraform accepted one while the hook did not, the two would disagree about
  # whether the migration is finished.
  _mc_enabled      = try(var.settings.spec.enabled, false) == true
  _mc_has_settings = length(try(keys(var.settings.spec.settings), [])) > 0

  _credential_secrets_in = {
    for key, s in var.secrets : try(s.metadata.name, key) => s
    if try(s.type, "") == local._credentials_secret_type
  }

  # Both Go surfaces require the primary Secret by name — IsMigrationResourcesApplied() in
  # hooks/internal/migration.go and IsNewResourcesComplete() in
  # go_lib/cloud-provider/validation/api/migration.go both look up d8-credentials. Counting any
  # credentials-typed Secret would be wider: the NAT-instance exporter Secret carries the same
  # type, so a cluster with only that one would look migrated here and unmigrated to the hook.
  _has_credential_secret = anytrue([
    for key, s in local._credential_secrets_in : try(s.metadata.name, key) == local._credentials_name
  ])

  # Every PCC node group must already have both a NodeGroup and its
  # YandexInstanceClass in the cluster before the new resources take over.
  # Mirrors IsMigrationResourcesApplied() in hooks/internal/migration.go: a
  # half-applied migration must keep reading the PCC, otherwise terraform would
  # flap between two incomplete sources.
  # No length() guard on purpose: a PCC without node groups has nothing left to apply, and the
  # hook agrees — its loop over the PCC node groups simply does not run. Requiring at least one
  # kept such a cluster reading the PCC forever while the hook considered the migration done.
  _pcc_resources_applied = alltrue([
    for name in keys(local._pcc_ngs_by_name) : (
      name != "" &&
      try(var.nodeGroups[name], null) != null &&
      try(var.instanceClasses[
        try(var.nodeGroups[name].spec.cloudInstances.classReference.name, local._instance_class_names[name])
      ], null) != null
    )
  ])

  new_resources_complete = (
    local._mc_version >= 2 &&
    local._mc_enabled &&
    local._mc_has_settings &&
    local._has_credential_secret &&
    local._pcc_resources_applied
  )

  # Use the PCC while it is present and the new resources are not complete yet.
  use_pcc = local.has_pcc && !local.new_resources_complete

  # --- Synthesised ModuleConfig ---------------------------------------------

  # Mirrors BuildModuleConfigSettingsV2(): the whole withNATInstance block moves
  # into nodes.parameters, so the WithNATInstance layout keeps its subnets,
  # addresses and instance sizing after the migration.
  #
  # Every key is always present with a zero value rather than omitted: terraform
  # requires both branches of a conditional to have the same type, and the
  # nodeParameters output normalises empty strings back to null anyway.
  _pcc_with_nat_instance = {
    externalSubnetID           = try(local._pcc.withNATInstance.externalSubnetID, "")
    internalSubnetID           = try(local._pcc.withNATInstance.internalSubnetID, "")
    internalSubnetCIDR         = try(local._pcc.withNATInstance.internalSubnetCIDR, "")
    natInstanceExternalAddress = try(local._pcc.withNATInstance.natInstanceExternalAddress, "")
    natInstanceInternalAddress = try(local._pcc.withNATInstance.natInstanceInternalAddress, "")
    natInstanceResources = {
      cores    = try(local._pcc.withNATInstance.natInstanceResources.cores, 2)
      memory   = try(local._pcc.withNATInstance.natInstanceResources.memory, 2048)
      platform = try(local._pcc.withNATInstance.natInstanceResources.platform, "standard-v2")
    }
  }

  _pcc_dhcp_options = {
    domainName        = try(local._pcc.dhcpOptions.domainName, "")
    domainNameServers = try(tolist(local._pcc.dhcpOptions.domainNameServers), [])
  }

  # externalIPAddresses and externalSubnetIDs are per-node-group lists in the
  # PCC instanceClass. YandexInstanceClass has no externalIPAddresses
  # counterpart, so they are keyed by node group name in nodes.parameters.
  _pcc_external_ip_addresses = {
    for ng in local._pcc_ngs : ng.name => ng.instanceClass.externalIPAddresses
    if length(try(ng.instanceClass.externalIPAddresses, [])) > 0
  }

  _pcc_external_subnet_ids = {
    for ng in local._pcc_ngs : ng.name => ng.instanceClass.externalSubnetIDs
    if length(try(ng.instanceClass.externalSubnetIDs, [])) > 0
  }

  _pcc_module_config = {
    apiVersion = "deckhouse.io/v1alpha1"
    kind       = "ModuleConfig"
    metadata   = { name = "cloud-provider-yandex" }
    spec = {
      enabled = true
      version = 2
      settings = {
        provider = {
          parameters = {
            cloudID  = try(local._pcc.provider.cloudID, "")
            folderID = try(local._pcc.provider.folderID, "")
          }
        }
        nodes = {
          disabled = false
          parameters = {
            sshPublicKey              = try(local._pcc.sshPublicKey, "")
            layout                    = try(local._pcc.layout, "")
            nodeNetworkCIDR           = try(local._pcc.nodeNetworkCIDR, "")
            zones                     = local._pcc_zones
            existingNetworkID         = try(local._pcc.existingNetworkID, "")
            existingZoneToSubnetIDMap = try(tomap(local._pcc.existingZoneToSubnetIDMap), {})
            labels                    = try(tomap(local._pcc.labels), {})
            externalIPAddresses       = local._pcc_external_ip_addresses
            externalSubnetIDs         = local._pcc_external_subnet_ids
            withNATInstance           = local._pcc_with_nat_instance
            dhcpOptions               = local._pcc_dhcp_options
          }
        }
        # storage and ccm carry no parameters here on purpose. They come from ModuleConfig v1
        # (storageClass.exclude, additionalExternalNetworkIDs), which terraform never receives —
        # var.settings holds the v2 ModuleConfig only. Nothing on the terraform side reads these
        # two sections either: only nodes.parameters drives the infrastructure. The hook projects
        # them from MC v1 for in-cluster consumers, see BuildModuleConfigSettingsV2() in
        # hooks/internal/migration.go.
        storage = {
          disabled   = false
          parameters = {}
        }
        ccm = {
          disabled   = false
          parameters = {}
        }
      }
    }
  }

  # --- Synthesised NodeGroups ----------------------------------------------

  # resolveZones(): node group zones win, then the cluster-wide zones, and an
  # empty result omits the key so node-manager falls back to defaultZones.
  _pcc_resolved_zones = {
    for ng in local._pcc_ngs : ng.name => compact(
      length(ng.zones) > 0 ? ng.zones : local._pcc_zones
    )
  }

  _pcc_node_groups = {
    for ng in local._pcc_ngs : ng.name => {
      apiVersion = "deckhouse.io/v1"
      kind       = "NodeGroup"
      metadata   = { name = ng.name }
      spec = {
        nodeType = "CloudPermanent"
        cloudInstances = {
          classReference = {
            kind = "YandexInstanceClass"
            name = local._instance_class_names[ng.name]
          }
          minPerZone = ng.replicas
          maxPerZone = ng.replicas
          # null means "no explicit zones": consumers then use every zone the
          # subnets cover, matching node-manager's defaultZones fallback.
          zones = length(local._pcc_resolved_zones[ng.name]) > 0 ? local._pcc_resolved_zones[ng.name] : null
        }
        nodeTemplate = ng.nodeTemplate
      }
    }
  }

  # --- Synthesised YandexInstanceClasses ------------------------------------

  _pcc_instance_classes = {
    for ng in local._pcc_ngs : local._instance_class_names[ng.name] => {
      apiVersion = "deckhouse.io/v1"
      kind       = "YandexInstanceClass"
      metadata   = { name = local._instance_class_names[ng.name] }
      spec = {
        cores        = try(ng.instanceClass.cores, 0)
        memory       = try(ng.instanceClass.memory, 0)
        imageID      = try(ng.instanceClass.imageID, "")
        platformID   = try(ng.instanceClass.platform, local._default_platform_id)
        diskType     = try(ng.instanceClass.diskType, local._default_disk_type)
        diskSizeGB   = try(ng.instanceClass.diskSizeGB, local._default_disk_size_gb)
        coreFraction = try(ng.instanceClass.coreFraction, 100)
        networkType  = try(ng.instanceClass.networkType, "Standard")
        # A public address is requested through nodes.parameters.externalIPAddresses
        # when migrating from a PCC; the boolean covers ModuleConfig-only clusters.
        assignPublicIPAddress = false
        additionalLabels      = try(ng.instanceClass.additionalLabels, {})
        additionalSubnets     = try(ng.instanceClass.externalSubnetIDs, [])
        # The deprecated single externalSubnetID becomes the primary NIC subnet.
        mainSubnet = try(ng.instanceClass.externalSubnetID, "")
        # etcd lives on a dedicated disk on master nodes only.
        etcdDiskSizeGB = ng.name == "master" ? try(local._pcc.masterNodeGroup.instanceClass.etcdDiskSizeGb, local._default_etcd_disk_size_gb) : null
      }
    }
  }

  # --- Synthesised credential Secrets --------------------------------------

  # Keyed as "<namespace>/<name>" so the map shape matches what dhctl passes in
  # State B (see fetchCredentialSecretsFromCluster in dhctl).
  _pcc_credential_secret_list = concat(
    try(local._pcc.provider.serviceAccountJSON, "") != "" ? [{
      name       = local._credentials_name
      authScheme = "serviceAccount"
      secret     = local._pcc.provider.serviceAccountJSON
    }] : [],
    try(local._pcc.withNATInstance.exporterAPIKey, "") != "" ? [{
      name       = local._exporter_credentials_name
      authScheme = "apiToken"
      secret     = local._pcc.withNATInstance.exporterAPIKey
    }] : [],
  )

  _pcc_credential_secrets = {
    for s in local._pcc_credential_secret_list : "${local._namespace}/${s.name}" => {
      apiVersion = "v1"
      kind       = "Secret"
      metadata = {
        name      = s.name
        namespace = local._namespace
      }
      stringData = {
        authScheme = s.authScheme
        secret     = s.secret
      }
      type = local._credentials_secret_type
    }
  }

  # --- Resolved values ------------------------------------------------------

  # The synthesised objects and the cluster objects never have identical static
  # types, so the choice is made on the JSON encoding and decoded back into a
  # single dynamic value.
  resolved_settings         = jsondecode(local.use_pcc ? jsonencode(local._pcc_module_config) : jsonencode(var.settings))
  resolved_node_groups      = jsondecode(local.use_pcc ? jsonencode(local._pcc_node_groups) : jsonencode(var.nodeGroups))
  resolved_instance_classes = jsondecode(local.use_pcc ? jsonencode(local._pcc_instance_classes) : jsonencode(var.instanceClasses))
  resolved_secrets          = jsondecode(local.use_pcc ? jsonencode(local._pcc_credential_secrets) : jsonencode(var.secrets))

  _node_params = try(local.resolved_settings.spec.settings.nodes.parameters, {})

  # Credential Secrets keyed by bare name, accepting both the "<ns>/<name>" keys
  # dhctl produces and plain names, and both stringData and base64 data.
  _resolved_credentials = {
    for key, s in local.resolved_secrets : try(s.metadata.name, key) => try(
      s.stringData.secret,
      base64decode(try(s.data.secret, "")),
      "",
    )
    if try(s.type, "") == local._credentials_secret_type
  }

  # --- Validation ------------------------------------------------------------

  # Nothing to validate when neither source is supplied: this is a destroy or a
  # not-yet-configured run and every consumer degrades to empty values.
  _configured = local.has_pcc || var.settings != null

  _network_cidr  = try(local._node_params.nodeNetworkCIDR, "")
  _layout        = try(local._node_params.layout, "")
  _known_layouts = ["Standard", "WithoutNAT", "WithNATInstance"]

  # Every NodeGroup must point at a YandexInstanceClass that actually exists,
  # otherwise the node modules would silently plan a machine with zeroed cores.
  _dangling_class_references = [
    for name, ng in local.resolved_node_groups : name
    if try(local.resolved_instance_classes[ng.spec.cloudInstances.classReference.name], null) == null
  ]

  _wrong_kind_class_references = [
    for name, ng in local.resolved_node_groups : name
    if try(ng.spec.cloudInstances.classReference.kind, "YandexInstanceClass") != "YandexInstanceClass"
  ]

  _empty_pcc_ng_names = length([for name in local._pcc_ng_names : name if name == ""])

  _duplicate_pcc_ng_names = length(local._pcc_ng_names) != length(distinct(local._pcc_ng_names))
}
