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

variable "clusterConfiguration" {
  type = any
}

locals {
  project_namespace = try(var.providerClusterConfiguration.provider.namespace, "")

  network_policy_raw = try(var.providerClusterConfiguration.provider.networkPolicy, "Isolated")
  network_policy_mode = (
    local.network_policy_raw == null || trimspace(tostring(local.network_policy_raw)) == ""
  ) ? "Isolated" : tostring(local.network_policy_raw)

  should_check_project = local.project_namespace != "" && local.network_policy_mode == "Isolated"

  cluster_prefix = try(var.clusterConfiguration.cloud.prefix, "")

  should_manage_np = local.should_check_project && local.cluster_prefix != ""
}

locals {
  template_ingress = [
    {
      from = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = local.project_namespace }
        }
      }]
    },
    {
      from  = [{ ipBlock = { cidr = "0.0.0.0/0" } }]
      ports = [{ port = 22, protocol = "TCP" }]
    },
    {
      from = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "d8-ingress-nginx" }
        }
        podSelector = {
          matchLabels = { app = "controller" }
        }
      }]
    },
    {
      from = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "d8-metallb" }
        }
      }]
    }
  ]

  template_egress = [
    {
      to = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = local.project_namespace }
        }
      }]
    },
    { to = [{ ipBlock = { cidr = "0.0.0.0/0" } }] },
    {
      to = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "d8-ingress-nginx" }
        }
        podSelector = {
          matchLabels = { app = "controller" }
        }
      }]
    },
    {
      ports = [{ port = 53, protocol = "UDP" }]
      to = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "kube-system" }
        }
      }]
    },
    {
      to = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "d8-metallb" }
        }
      }]
    }
  ]
}

locals {
  targets = local.should_manage_np ? {
    isolated_cluster_prefix = {
      namespace = local.project_namespace
      name      = "isolated-${local.cluster_prefix}"
    }
  } : {}
}
