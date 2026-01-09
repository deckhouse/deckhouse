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

data "kubernetes_resources" "project_namespace" {
  count       = local.should_check_project ? 1 : 0
  api_version = "v1"
  kind        = "Namespace"

  field_selector = "metadata.name=${local.project_namespace}"
}

data "kubernetes_resources" "isolated_cluster_prefix_np" {
  count       = local.should_manage_np ? 1 : 0
  api_version = "networking.k8s.io/v1"
  kind        = "NetworkPolicy"
  namespace   = local.project_namespace

  field_selector = "metadata.name=isolated-${local.cluster_prefix}"
}

import {
  for_each = local.import_targets
  to       = kubernetes_manifest.isolated_cluster_prefix_network_policy[each.key]
  id       = "apiVersion=networking.k8s.io/v1,kind=NetworkPolicy,namespace=${each.value.namespace},name=${each.value.name}"
}

resource "kubernetes_manifest" "isolated_cluster_prefix_network_policy" {
  for_each = local.targets

  field_manager {
    force_conflicts = true
  }

  manifest = {
    apiVersion = "networking.k8s.io/v1"
    kind       = "NetworkPolicy"

    metadata = {
      name      = each.value.name
      namespace = each.value.namespace
      labels = {
        "d8.tf/managed" = "isolated_cluster_prefix"
        "d8.tf/hash"    = local.desired_policy_hash_short
      }
    }

    spec = {
      podSelector = {
        matchLabels = {
          "dvp.deckhouse.io/cluster-prefix" = local.cluster_prefix
        }
      }

      policyTypes = ["Ingress", "Egress"]
      ingress     = local.template_ingress
      egress      = local.template_egress
    }
  }
}

output "dvp_np_status" {
  value = {
    project_namespace     = local.project_namespace
    network_policy_mode   = local.network_policy_mode
    should_check_project  = local.should_check_project
    namespace_exists      = local.ns_exists
    cluster_prefix        = local.cluster_prefix
    should_manage_np      = local.should_manage_np

    isolated_np_exists    = local.isolated_np_exists
    will_import           = length(local.import_targets) > 0
    will_create_or_update = length(local.targets) > 0

    policy_hash_short     = local.desired_policy_hash_short
    ingress_rules_count   = length(local.template_ingress)
    egress_rules_count    = length(local.template_egress)
  }
}
