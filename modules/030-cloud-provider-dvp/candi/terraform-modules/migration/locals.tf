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

  _mc_version        = try(var.settings.spec.version, 0)
  _master_class_name = try(var.nodeGroups["master"].spec.cloudInstances.classReference.name, "")

  _has_master_ng = try(var.nodeGroups["master"], null) != null
  _has_master_ic = local._master_class_name != "" && try(var.instanceClasses[local._master_class_name], null) != null
  _has_credential_secret = try(
    length([for _, s in var.secrets : s if try(s.type, "") == "cloud-provider.deckhouse.io/credentials"]),
    0
  ) > 0

  # New resources are considered complete only when MC version >= 2, master NodeGroup,
  # corresponding DVPInstanceClass, and a credential Secret are all present.
  new_resources_complete = local._mc_version >= 2 && local._has_master_ng && local._has_master_ic && local._has_credential_secret

  # Use PCC when it is present and new resources are not yet complete (migration in progress).
  use_pcc = local.has_pcc && !local.new_resources_complete

  # --- PCC shorthand ---

  _pcc = var.providerClusterConfiguration

  # --- Synthesised module_config ---

  _pcc_module_config = {
    apiVersion = "deckhouse.io/v1alpha1"
    kind       = "ModuleConfig"
    metadata   = { name = "cloud-provider-dvp" }
    spec = {
      enabled = true
      version = 2
      settings = {
        provider = {
          parameters = {
            namespace     = try(local._pcc.provider.namespace, "")
            networkPolicy = try(local._pcc.provider.networkPolicy, "Isolated")
          }
        }
        nodes = {
          parameters = {
            sshPublicKey = try(local._pcc.sshPublicKey, "")
            region       = try(local._pcc.region, "")
            zones        = try(local._pcc.zones, [])
          }
        }
      }
    }
  }

  # --- Synthesised node_groups map ---

  # Build a unified list of all PCC node groups: master + workers.
  _pcc_all_ngs_list = concat(
    [
      {
        name          = "master"
        replicas      = try(local._pcc.masterNodeGroup.replicas, 1)
        zones         = try(local._pcc.masterNodeGroup.zones, null)
        instanceClass = try(local._pcc.masterNodeGroup.instanceClass, {})
      }
    ],
    [
      for ng in try(local._pcc.nodeGroups, []) : {
        name          = ng.name
        replicas      = try(ng.replicas, 1)
        zones         = try(ng.zones, null)
        instanceClass = try(ng.instanceClass, {})
      }
    ]
  )

  _pcc_node_groups = {
    for ng in local._pcc_all_ngs_list : ng.name => {
      apiVersion = "deckhouse.io/v1"
      kind       = "NodeGroup"
      metadata   = { name = ng.name }
      spec = {
        cloudInstances = {
          classReference = {
            kind = "DVPInstanceClass"
            name = "${ng.name}-dvp"
          }
          minPerZone = ng.replicas
          maxPerZone = ng.replicas
          zones      = ng.zones != null ? ng.zones : try(local._pcc.zones, [])
        }
        nodeType = "CloudPermanent"
      }
    }
  }

  # --- Synthesised instance_classes map ---

  # Strip ipAddresses from virtualMachine — it is not part of DVPInstanceClass.spec.
  _pcc_instance_classes = {
    for ng in local._pcc_all_ngs_list : "${ng.name}-dvp" => {
      apiVersion = "deckhouse.io/v1alpha1"
      kind       = "DVPInstanceClass"
      metadata   = { name = "${ng.name}-dvp" }
      spec = merge(
        ng.instanceClass,
        {
          virtualMachine = {
            for k, v in try(ng.instanceClass.virtualMachine, {}) : k => v
            if k != "ipAddresses"
          }
        }
      )
    }
  }

  # --- Synthesised credential_secrets map ---

  _pcc_credential_secrets = {
    "d8-credentials" = {
      apiVersion = "v1"
      kind       = "Secret"
      metadata = {
        name      = "d8-credentials"
        namespace = try(local._pcc.provider.namespace, "")
      }
      stringData = {
        authScheme = "kubeconfig"
        secret     = try(local._pcc.provider.kubeconfigDataBase64, "")
      }
      type = "cloud-provider.deckhouse.io/credentials"
    }
  }
}
