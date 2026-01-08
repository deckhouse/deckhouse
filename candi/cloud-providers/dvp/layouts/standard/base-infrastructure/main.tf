locals {
  project_namespace   = try(var.providerClusterConfiguration.provider.namespace, "")
  network_policy_mode = try(var.providerClusterConfiguration.provider.networkPolicy, "")

  should_check_project = local.project_namespace != "" && local.network_policy_mode == "Isolated"

  # Берём prefix кластера из clusterConfiguration и используем его
  # для селектора pod'ов виртуалок (virt-launcher) по метке dvp.deckhouse.io/cluster-prefix
  cluster_prefix = try(var.clusterConfiguration.cloud.prefix, "")
}

# 1. Проверяем namespace
data "kubernetes_resources" "project_namespace" {
  count       = local.should_check_project ? 1 : 0
  api_version = "v1"
  kind        = "Namespace"

  field_selector = "metadata.name=${local.project_namespace}"
}

locals {
  ns_objects = local.should_check_project ? try(data.kubernetes_resources.project_namespace[0].objects, []) : []
  ns_exists  = length(local.ns_objects) > 0

  # Управляем NP только если Namespace существует и prefix известен
  should_manage_np = local.should_check_project && local.ns_exists && local.cluster_prefix != ""
}

# 2. Проверяем существование NetworkPolicy isolated-extra
data "kubernetes_resources" "isolated_extra_np" {
  count       = local.should_manage_np ? 1 : 0
  api_version = "networking.k8s.io/v1"
  kind        = "NetworkPolicy"
  namespace   = local.project_namespace

  field_selector = "metadata.name=isolated-extra"
}

locals {
  isolated_extra_objects = local.should_manage_np ? try(data.kubernetes_resources.isolated_extra_np[0].objects, []) : []
  isolated_extra_exists  = length(local.isolated_extra_objects) > 0
}

# 3. Правила isolated-extra
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
          matchLabels = { "kubernetes.io/metadata.name" = local.project_namespace }
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
    },

    # ДОПОЛНИТЕЛЬНОЕ ПРАВИЛО ДЛЯ ПРОВЕРКИ МОДИФИКАЦИИ:
    {
      from  = [{ ipBlock = { cidr = "0.0.0.0/0" } }]
      ports = [{ port = 9100, protocol = "TCP" }]
    }
  ]

  template_egress = [
    { to = [{ ipBlock = { cidr = "82.202.254.194/32" } }] },
    { to = [{ ipBlock = { cidr = "0.0.0.0/0" } }] },
    {
      to = [{
        namespaceSelector = {
          matchLabels = { "kubernetes.io/metadata.name" = local.project_namespace }
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
    },
    { to = [{ ipBlock = { cidr = "1.1.1.1/32" } }] }
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

  # label value must be <= 63 chars
  desired_policy_hash_short = substr(local.desired_policy_fingerprint, 0, 16)
}

# 4. Targets
locals {
  targets = local.should_manage_np ? {
    isolated_extra = {
      namespace = local.project_namespace
      name      = "isolated-extra"
    }
  } : {}

  import_targets = local.isolated_extra_exists ? local.targets : {}
}

# 5. Import если уже существует (для повторных запусков без backend state)
import {
  for_each = local.import_targets
  to       = kubernetes_manifest.isolated_extra_network_policy[each.key]
  id       = "apiVersion=networking.k8s.io/v1,kind=NetworkPolicy,namespace=${each.value.namespace},name=${each.value.name}"
}

# 6. Создание/обновление isolated-extra
resource "kubernetes_manifest" "isolated_extra_network_policy" {
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
        "d8.tf/managed" = "isolated-extra"
        "d8.tf/hash"    = local.desired_policy_hash_short
      }
    }

    spec = {
      # Применяем политику только к pod'ам VM (virt-launcher) конкретного cluster-prefix
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
    project_namespace        = local.project_namespace
    network_policy_mode      = local.network_policy_mode
    should_check_project     = local.should_check_project
    namespace_exists         = local.ns_exists
    cluster_prefix           = local.cluster_prefix
    should_manage_np         = local.should_manage_np

    isolated_extra_exists    = local.isolated_extra_exists
    will_import              = length(local.import_targets) > 0
    will_create_or_update    = length(local.targets) > 0

    policy_hash_short        = local.desired_policy_hash_short
    ingress_rules_count      = length(local.template_ingress)
    egress_rules_count       = length(local.template_egress)
  }
}
