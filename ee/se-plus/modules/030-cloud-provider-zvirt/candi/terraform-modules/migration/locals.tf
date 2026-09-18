# Copyright 2026 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

# This module resolves a single source of truth for the terraform layouts.
#
# Two cluster states are supported:
#
#   State A (migration pending): ZvirtClusterConfiguration is still the source of truth. The
#     legacy configuration is projected onto the new resource shapes (ModuleConfig v2, NodeGroup,
#     ZvirtInstanceClass, credential Secret) so every consumer reads the new shapes only.
#   State B (migration done): the cluster resources are the source of truth and are passed
#     through unchanged.
#
# The projection must produce exactly what hooks/internal/migration.go writes into the cluster,
# otherwise terraform sees drift right after the in-cluster migration completes and recreates
# nodes.

locals {
  # --- Constants -------------------------------------------------------------

  _credentials_secret_type   = "cloud-provider.deckhouse.io/credentials"
  _namespace                 = "d8-cloud-provider-zvirt"
  _credentials_name          = "d8-credentials"
  _module_name               = "cloud-provider-zvirt"
  _instance_class_kind       = "ZvirtInstanceClass"
  _default_layout            = "Standard"
  _known_layouts             = ["Standard"]
  _default_root_disk_size_gb = 50
  _default_etcd_disk_size_gb = 10

  # --- PCC shorthand ---------------------------------------------------------

  _pcc                  = var.providerClusterConfiguration
  _pcc_node_group_count = try(length(local._pcc.nodeGroups), 0)
  has_pcc               = local._pcc != null && length(try(keys(local._pcc), [])) > 0

  # --- Node group inventory --------------------------------------------------

  _pcc_all_ngs_list = concat(
    try(local._pcc.masterNodeGroup.replicas, 0) > 0 ? [
      {
        name          = "master"
        replicas      = local._pcc.masterNodeGroup.replicas
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
        instanceClass = try(local._pcc.nodeGroups[i].instanceClass, {})
        nodeTemplate  = try(local._pcc.nodeGroups[i].nodeTemplate, null)
      }
    ]
  )

  _pcc_ng_names    = [for ng in local._pcc_all_ngs_list : ng.name]
  _pcc_ngs_by_name = { for ng in local._pcc_all_ngs_list : ng.name => ng... }
  _pcc_ngs         = [for name, group in local._pcc_ngs_by_name : group[0]]

  # Mirrors go_lib/cloud-provider/api.BuildInstanceClassName: a readable prefix capped at 50
  # characters with trailing dashes trimmed, plus a 48-bit SHA-256 suffix, keeping the name
  # inside the 63-character DNS-1123 label limit.
  _instance_class_names = {
    for name in keys(local._pcc_ngs_by_name) : name => format(
      "%s-%s",
      replace(substr(name, 0, 50), "/-+$/", ""),
      substr(sha256(name), 0, 12),
    )
  }

  # --- Source-of-truth detection --------------------------------------------

  _mc_version      = try(var.settings.spec.version, 0)
  _mc_enabled      = try(var.settings.spec.enabled, false) == true
  _mc_has_settings = length(try(keys(var.settings.spec.settings), [])) > 0

  _credential_secrets_in = {
    for key, s in var.secrets : try(s.metadata.name, key) => s
    if try(s.type, "") == local._credentials_secret_type
  }

  _has_credential_secret = anytrue([
    for key, s in local._credential_secrets_in : try(s.metadata.name, key) == local._credentials_name
  ])

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

  use_pcc = local.has_pcc && !local.new_resources_complete

  # --- Synthesised ModuleConfig ---------------------------------------------

  _pcc_custom_network_configs = {
    for ng in local._pcc_ngs : ng.name => {
      networkInterfaceName      = try(ng.instanceClass.customNetworkConfig.networkInterfaceName, "")
      networkInterfaceAddresses = try(tolist(ng.instanceClass.customNetworkConfig.networkInterfaceAddress), [])
      networkInterfaceNetmask   = try(ng.instanceClass.customNetworkConfig.networkInterfaceNetmask, "")
      networkInterfaceGateway   = try(ng.instanceClass.customNetworkConfig.networkInterfaceGateway, "")
      dnsServers                = compact(split(" ", try(ng.instanceClass.customNetworkConfig.dnsServers, "")))
    } if try(ng.instanceClass.customNetworkConfig, null) != null
  }

  _pcc_node_parameters = merge(
    {
      sshPublicKey = try(local._pcc.sshPublicKey, "")
      layout       = try(local._pcc.layout, local._default_layout)
    },
    length(local._pcc_custom_network_configs) > 0 ? {
      customNetworkConfigs = local._pcc_custom_network_configs
    } : {}
  )

  _pcc_module_config = {
    apiVersion = "deckhouse.io/v1alpha1"
    kind       = "ModuleConfig"
    metadata   = { name = local._module_name }
    spec = {
      enabled = true
      version = 2
      settings = {
        provider = {
          parameters = {
            server    = try(local._pcc.provider.server, "")
            clusterID = try(local._pcc.clusterID, "")
            caBundle  = try(local._pcc.provider.caBundle, "")
            insecure  = try(local._pcc.provider.insecure, false)
          }
        }
        nodes = {
          parameters = local._pcc_node_parameters
        }
      }
    }
  }

  # --- Synthesised NodeGroups ----------------------------------------------

  _pcc_node_groups = {
    for ng in local._pcc_ngs : ng.name => {
      apiVersion = "deckhouse.io/v1"
      kind       = "NodeGroup"
      metadata   = { name = ng.name }
      spec = {
        nodeType = "CloudPermanent"
        cloudInstances = {
          classReference = {
            kind = local._instance_class_kind
            name = local._instance_class_names[ng.name]
          }
          minPerZone = ng.replicas
          maxPerZone = ng.replicas
        }
        nodeTemplate = ng.nodeTemplate
      }
    }
  }

  # --- Synthesised ZvirtInstanceClasses -------------------------------------

  _pcc_instance_classes = {
    for ng in local._pcc_ngs : local._instance_class_names[ng.name] => {
      apiVersion = "deckhouse.io/v1"
      kind       = local._instance_class_kind
      metadata   = { name = local._instance_class_names[ng.name] }
      spec = {
        numCPUs         = try(ng.instanceClass.numCPUs, 0)
        memory          = try(ng.instanceClass.memory, 0)
        template        = try(ng.instanceClass.template, "")
        vnicProfileID   = try(ng.instanceClass.vnicProfileID, "")
        storageDomainID = try(ng.instanceClass.storageDomainID, "")
        rootDiskSizeGb  = try(ng.instanceClass.rootDiskSizeGb, local._default_root_disk_size_gb)
        etcdDiskSizeGb  = ng.name == "master" ? try(local._pcc.masterNodeGroup.instanceClass.etcdDiskSizeGb, local._default_etcd_disk_size_gb) : null
      }
    }
  }

  # --- Synthesised credential Secrets --------------------------------------

  _pcc_credential_secrets = try(local._pcc.provider.username, "") == "" ? {} : {
    "${local._namespace}/${local._credentials_name}" = {
      apiVersion = "v1"
      kind       = "Secret"
      metadata = {
        name      = local._credentials_name
        namespace = local._namespace
      }
      stringData = {
        authScheme = "userPassword"
        identity   = local._pcc.provider.username
        secret     = try(local._pcc.provider.password, "")
      }
      type = local._credentials_secret_type
    }
  }

  # --- Resolved values ------------------------------------------------------

  resolved_settings         = jsondecode(local.use_pcc ? jsonencode(local._pcc_module_config) : jsonencode(var.settings))
  resolved_node_groups      = jsondecode(local.use_pcc ? jsonencode(local._pcc_node_groups) : jsonencode(var.nodeGroups))
  resolved_instance_classes = jsondecode(local.use_pcc ? jsonencode(local._pcc_instance_classes) : jsonencode(var.instanceClasses))
  resolved_secrets          = jsondecode(local.use_pcc ? jsonencode(local._pcc_credential_secrets) : jsonencode(var.secrets))

  _provider_params = try(local.resolved_settings.spec.settings.provider.parameters, {})
  _node_params     = try(local.resolved_settings.spec.settings.nodes.parameters, {})

  _resolved_credentials = {
    for key, s in local.resolved_secrets : try(s.metadata.name, key) => {
      identity = try(s.stringData.identity, base64decode(try(s.data.identity, "")), "")
      secret   = try(s.stringData.secret, base64decode(try(s.data.secret, "")), "")
    }
    if try(s.type, "") == local._credentials_secret_type
  }

  # --- Validation ------------------------------------------------------------

  _configured = local.has_pcc || var.settings != null

  _layout     = try(local._node_params.layout, "")
  _server     = try(local._provider_params.server, "")
  _cluster_id = try(local._provider_params.clusterID, "")

  _dangling_class_references = [
    for name, ng in local.resolved_node_groups : name
    if try(local.resolved_instance_classes[ng.spec.cloudInstances.classReference.name], null) == null
  ]

  _wrong_kind_class_references = [
    for name, ng in local.resolved_node_groups : name
    if try(ng.spec.cloudInstances.classReference.kind, local._instance_class_kind) != local._instance_class_kind
  ]

  _empty_pcc_ng_names     = length([for name in local._pcc_ng_names : name if name == ""])
  _duplicate_pcc_ng_names = length(local._pcc_ng_names) != length(distinct(local._pcc_ng_names))
}
