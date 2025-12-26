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

locals {
  project_name      = var.providerClusterConfiguration.provider.namespace
  project_namespace = var.providerClusterConfiguration.provider.namespace
  policy_name       = "isolated"
}

data "kubernetes_resources" "project" {
  api_version    = "deckhouse.io/v1alpha2"
  kind           = "Project"
  field_selector = "metadata.name=${local.project_name}"
}

data "kubernetes_resources" "namespace" {
  api_version    = "v1"
  kind           = "Namespace"
  field_selector = "metadata.name=${local.project_namespace}"
}

locals {
  project_exists    = length(data.kubernetes_resources.project.objects) > 0
  namespace_exists  = length(data.kubernetes_resources.namespace.objects) > 0
  project_network_policy = local.project_exists ? try(data.kubernetes_resources.project.objects[0].object.spec.parameters.networkPolicy, "") : ""
  should_manage     = local.project_exists && local.namespace_exists && local.project_network_policy == "Isolated"
}

data "kubernetes_resources" "isolated_np" {
  count       = local.should_manage ? 1 : 0
  api_version = "networking.k8s.io/v1"
  kind        = "NetworkPolicy"
  namespace   = local.project_namespace
  field_selector = "metadata.name=${local.policy_name}"
}

locals {
  isolated_np_exists = local.should_manage && length(try(data.kubernetes_resources.isolated_np[0].objects, [])) > 0

  existing_ingress = local.isolated_np_exists ? try(data.kubernetes_resources.isolated_np[0].objects[0].object.spec.ingress, []) : []
  existing_egress  = local.isolated_np_exists ? try(data.kubernetes_resources.isolated_np[0].objects[0].object.spec.egress, []) : []

  existing_ingress_keys = [for r in local.existing_ingress : jsonencode(r)]
  existing_egress_keys  = [for r in local.existing_egress : jsonencode(r)]

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
        { port = 30000, endPort = 32767, protocol = "TCP" },
      ]
    },
    { from = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = local.project_namespace } } }] },
    { from = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "d8-virtualization" } } }] },
    {
      from = [{
        namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "d8-monitoring" } }
        podSelector       = { matchLabels = { app = "prometheus" } }
      }]
    },
    {
      from = [{
        namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "d8-ingress-nginx" } }
        podSelector       = { matchLabels = { app = "controller" } }
      }]
    },
    { from = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "d8-commander" } } }] },
    { from = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "bastion" } } }] },
    { from = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "d8-openvpn" } } }] },
    { from = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "d8-metallb" } } }] },
  ]

  template_egress = [
    { to = [{ ipBlock = { cidr = "82.202.254.194/32" } }] },
    { to = [{ ipBlock = { cidr = "0.0.0.0/0" } }] },
    { to = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = local.project_namespace } } }] },
    { to = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "d8-virtualization" } } }] },
    {
      to = [{
        namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "d8-ingress-nginx" } }
        podSelector       = { matchLabels = { app = "controller" } }
      }]
    },
    {
      ports = [{ port = 53, protocol = "UDP" }]
      to    = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "kube-system" } } }]
    },
    { to = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "bastion" } } }] },
    { to = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "d8-openvpn" } } }] },
    { to = [{ namespaceSelector = { matchLabels = { "kubernetes.io/metadata.name" = "d8-metallb" } } }] },

    { to = [{ ipBlock = { cidr = "1.1.1.1/32" } }] },
  ]

  missing_ingress = [for r in local.template_ingress : r if !contains(local.existing_ingress_keys, jsonencode(r))]
  missing_egress  = [for r in local.template_egress  : r if !contains(local.existing_egress_keys,  jsonencode(r))]

  merged_ingress = concat(local.existing_ingress, local.missing_ingress)
  merged_egress  = concat(local.existing_egress,  local.missing_egress)
}

resource "kubernetes_manifest" "isolated_network_policy" {
  count = local.should_manage ? 1 : 0

  field_manager {
    force_conflicts = true
  }

  manifest = {
    apiVersion = "networking.k8s.io/v1"
    kind       = "NetworkPolicy"
    metadata = {
      name      = local.policy_name
      namespace = local.project_namespace
    }
    spec = {
      podSelector = {}
      policyTypes = ["Ingress", "Egress"]
      ingress     = local.merged_ingress
      egress      = local.merged_egress
    }
  }
}
