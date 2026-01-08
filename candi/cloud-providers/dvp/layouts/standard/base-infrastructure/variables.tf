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
  project_namespace   = try(var.providerClusterConfiguration.provider.namespace, "")
  network_policy_mode = try(var.providerClusterConfiguration.provider.networkPolicy, "")

  should_check_project = local.project_namespace != "" && local.network_policy_mode == "Isolated"

  cluster_prefix = try(var.clusterConfiguration.cloud.prefix, "")
}

locals {
  ns_objects = local.should_check_project ? try(data.kubernetes_resources.project_namespace[0].objects, []) : []
  ns_exists  = length(local.ns_objects) > 0

  should_manage_np = local.should_check_project && local.ns_exists && local.cluster_prefix != ""
}

locals {
  isolated_np_objects = local.should_manage_np ? try(data.kubernetes_resources.isolated_cluster_prefix_np[0].objects, []) : []
  isolated_np_exists  = length(local.isolated_np_objects) > 0
}

locals {
  template_ingress = [
    {
      from  = [{ ipBlock = { cidr = "0.0.0.0/0" } }]
      ports = [{ port = 22, protocol = "TCP" }]
    },
    {
      from = [{ ipBlock = { cidr = "0.0.0.0/0" } }]
      ports = [
        { port = 80, protocol = "TCP" },
        { port = 443, protocol = "TCP" },
        { port = 30000, endPort = 32767, protocol = "TCP" }
      ]
    },
    {
      from = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "team-d8-cloud-providers" }
        }
      }]
    },
    {
      from = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "d8-virtualization" }
        }
      }]
    },
    {
      from = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "d8-monitoring" }
        }
        podSelector = {
          matchLabels = { app = "prometheus" }
        }
      }]
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
          matchLabels = { "kubernetes.io/metadata.name" = "d8-commander" }
        }
      }]
    },
    {
      from = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "bastion" }
        }
      }]
    },
    {
      from = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "d8-openvpn" }
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
    { to = [{ ipBlock = { cidr = "0.0.0.0/0" } }] },
    {
      to = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "team-d8-cloud-providers" }
        }
      }]
    },
    {
      to = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "d8-virtualization" }
        }
      }]
    },
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
          matchLabels = { "kubernetes.io/metadata.name" = "bastion" }
        }
      }]
    },
    {
      to = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = "d8-openvpn" }
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

  desired_policy_fingerprint = sha256(jsonencode({
    podSelector = {
      matchLabels = {
        "dvp.deckhouse.io/cluster-prefix" = local.cluster_prefix
      }
    }
    ingress = local.template_ingress
    egress  = local.template_egress
  }))

  desired_policy_hash_short = substr(local.desired_policy_fingerprint, 0, 16)
}

locals {
  targets = local.should_manage_np ? {
    isolated_cluster_prefix = {
      namespace = local.project_namespace
      name      = "isolated-${local.cluster_prefix}"
    }
  } : {}

  import_targets = local.isolated_np_exists ? local.targets : {}
}