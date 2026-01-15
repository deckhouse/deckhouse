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

variable "providerClusterConfiguration" {
  type = any
}

variable "clusterPrefix" {
  type    = string
  default = ""
}

locals {
  project_namespace   = try(var.providerClusterConfiguration.provider.namespace, "")
  network_policy_mode = try(var.providerClusterConfiguration.provider.networkPolicy, "Isolated")

  should_check_project = local.project_namespace != "" && local.network_policy_mode == "Isolated"

  cluster_prefix = var.clusterPrefix

  should_manage_np = local.should_check_project && local.cluster_prefix != ""
}

locals {
  template_ingress = [
    {
      from = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "kube-system" }
        }
      }]
    },
    {
      from = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "d8-system" }
        }
      }]
    },
    {
      from = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = local.project_namespace }
        }
      }]
    },
  ]

  template_egress = [
    {
      to = [{
        ipBlock = {
          cidr = "0.0.0.0/0"
        }
      }]
    }
  ]
}

locals {
  targets = local.should_manage_np ? {
    "isolated-${local.cluster_prefix}" = {
      apiVersion = "networking.k8s.io/v1"
      kind       = "NetworkPolicy"
      metadata = {
        name      = "isolated-${local.cluster_prefix}"
        namespace = local.project_namespace
        labels = {
          "d8.tf/managed" = "isolated_cluster_prefix"
        }
      }
      spec = {
        podSelector = {}
        policyTypes = ["Ingress", "Egress"]
        ingress     = local.template_ingress
        egress      = local.template_egress
      }
    }
  } : {}
}