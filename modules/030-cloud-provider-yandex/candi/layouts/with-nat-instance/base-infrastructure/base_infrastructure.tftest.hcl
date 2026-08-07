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

# Run from this directory with the OpenTofu version pinned in
# candi/terraform_versions.yml:
#
#   tofu init -backend=false && tofu test
#
# override_resource/override_data target a whole resource, never a single
# instance: OpenTofu rejects `yandex_vpc_network.kube[0]` with "Resource instance
# address with keys is not allowed" (verified on v1.12.5). A non-indexed target
# overrides every instance of that resource, which is what these count = 1
# resources need anyway.
#
# See layouts/standard/base-infrastructure/base_infrastructure.tftest.hcl for why
# the provider is mocked, why vpc-components is overridden, and where the
# rejected-configuration scenarios live. monitoring-service-account is overridden
# as well: it provisions a service account whose API key is only known after
# apply.

mock_provider "yandex" {}

variables {
  clusterConfiguration         = { cloud = { prefix = "test-prefix" } }
  clusterUUID                  = "11111111-1111-1111-1111-111111111111"
  providerClusterConfiguration = null
  nodeGroups                   = {}
  instanceClasses              = {}
  secrets                      = {}
  settings                     = null
}

# State A with a fully specified NAT instance. Everything under withNATInstance
# has to reach vpc-components, otherwise converge rebuilds the NAT instance.
run "state_a_full_nat_configuration" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "WithNATInstance"
      sshPublicKey    = "ssh-rsa NAT_KEY"
      nodeNetworkCIDR = "10.30.0.0/16"
      zones           = ["ru-central1-a", "ru-central1-b"]
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8" }
      }
      withNATInstance = {
        exporterAPIKey             = "exporter-key"
        internalSubnetID           = "subnet-internal"
        internalSubnetCIDR         = "10.30.4.0/24"
        externalSubnetID           = "subnet-external"
        natInstanceExternalAddress = "203.0.113.10"
        natInstanceInternalAddress = "10.30.4.10"
        natInstanceResources = {
          cores    = 4
          memory   = 8192
          platform = "standard-v3"
        }
      }
      provider = {
        cloudID            = "cloud-nat"
        folderID           = "folder-nat"
        serviceAccountJSON = "{\"id\":\"sa-nat\"}"
      }
    }
  }

  override_module {
    target = module.vpc_components
    outputs = {
      route_table_id = "route-table-nat"
      zone_to_subnet_id_map = {
        "ru-central1-a" = "subnet-a"
        "ru-central1-b" = "subnet-b"
        "ru-central1-d" = "subnet-d"
      }
      nat_instance_name = "test-prefix-nat"
      nat_instance_zone = "ru-central1-a"
    }
  }

  override_module {
    target  = module.monitoring-service-account
    outputs = { apiKey = "exporter-key" }
  }

  override_resource {
    target = yandex_vpc_network.kube
    values = { id = "network-created" }
  }

  assert {
    condition     = output.cloud_discovery_data.natInstanceName == "test-prefix-nat"
    error_message = "expected the NAT instance name to be published in the discovery data"
  }

  assert {
    condition     = output.cloud_discovery_data.natInstanceZone == "ru-central1-a"
    error_message = "expected the NAT instance zone to be published in the discovery data"
  }

  assert {
    condition     = output.cloud_discovery_data.monitoringAPIKey == "exporter-key"
    error_message = "expected the monitoring API key to be published in the discovery data"
  }

  assert {
    condition     = jsonencode(output.cloud_discovery_data.zones) == jsonencode(["ru-central1-a", "ru-central1-b"])
    error_message = "expected the discovered zones to be intersected with the configured zones"
  }

  # The NAT instance provides egress, so nodes do not need public addresses.
  assert {
    condition     = output.cloud_discovery_data.shouldAssignPublicIPAddress == false
    error_message = "expected the WithNATInstance layout not to require public addresses on nodes"
  }
}

# A minimal withNATInstance block: vpc-components has to fall back to its own
# defaults without the layout passing nulls where a value is required.
run "state_a_minimal_nat_configuration" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "WithNATInstance"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.30.0.0/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8" }
      }
      withNATInstance = {}
      provider = {
        cloudID            = "cloud-nat"
        folderID           = "folder-nat"
        serviceAccountJSON = "{\"id\":\"sa-nat\"}"
      }
    }
  }

  override_module {
    target = module.vpc_components
    outputs = {
      route_table_id        = "route-table-nat"
      zone_to_subnet_id_map = { "ru-central1-a" = "subnet-a" }
      nat_instance_name     = "test-prefix-nat"
      nat_instance_zone     = "ru-central1-a"
    }
  }

  # No exporter key: the monitoring service account is not provisioned and the
  # module passes the empty key through.
  override_module {
    target  = module.monitoring-service-account
    outputs = { apiKey = "" }
  }

  override_resource {
    target = yandex_vpc_network.kube
    values = { id = "network-created" }
  }

  assert {
    condition     = output.cloud_discovery_data.monitoringAPIKey == ""
    error_message = "expected no monitoring API key when the configuration has none"
  }
}

# State B: no YandexClusterConfiguration. The exporter key now comes from the
# d8-credentials-exporter Secret, not from the ModuleConfig.
run "state_b_module_config_and_exporter_secret" {
  command = plan

  variables {
    nodeGroups = {
      master = {
        apiVersion = "deckhouse.io/v1"
        kind       = "NodeGroup"
        metadata   = { name = "master" }
        spec = {
          nodeType = "CloudPermanent"
          cloudInstances = {
            classReference = { kind = "YandexInstanceClass", name = "master-fc613b4dfd67" }
            minPerZone     = 1
            maxPerZone     = 1
          }
        }
      }
    }

    instanceClasses = {
      "master-fc613b4dfd67" = {
        apiVersion = "deckhouse.io/v1"
        kind       = "YandexInstanceClass"
        metadata   = { name = "master-fc613b4dfd67" }
        spec       = { cores = 4, memory = 8192, imageID = "fd8" }
      }
    }

    secrets = {
      "d8-cloud-provider-yandex/d8-credentials" = {
        apiVersion = "v1"
        kind       = "Secret"
        metadata   = { name = "d8-credentials", namespace = "d8-cloud-provider-yandex" }
        stringData = { authScheme = "serviceAccount", secret = "{\"id\":\"sa\"}" }
        type       = "cloud-provider.deckhouse.io/credentials"
      }
      "d8-cloud-provider-yandex/d8-credentials-exporter" = {
        apiVersion = "v1"
        kind       = "Secret"
        metadata   = { name = "d8-credentials-exporter", namespace = "d8-cloud-provider-yandex" }
        stringData = { authScheme = "apiToken", secret = "exporter-from-secret" }
        type       = "cloud-provider.deckhouse.io/credentials"
      }
    }

    settings = {
      apiVersion = "deckhouse.io/v1alpha1"
      kind       = "ModuleConfig"
      metadata   = { name = "cloud-provider-yandex" }
      spec = {
        enabled = true
        version = 2
        settings = {
          provider = { parameters = { cloudID = "cloud-b", folderID = "folder-b" } }
          nodes = {
            parameters = {
              sshPublicKey    = "ssh-rsa STATE_B"
              layout          = "WithNATInstance"
              nodeNetworkCIDR = "10.70.0.0/16"
              withNATInstance = {
                internalSubnetCIDR = "10.70.4.0/24"
                natInstanceResources = {
                  cores    = 2
                  memory   = 4096
                  platform = "standard-v2"
                }
              }
            }
          }
        }
      }
    }
  }

  override_module {
    target = module.vpc_components
    outputs = {
      route_table_id        = "route-table-b"
      zone_to_subnet_id_map = { "ru-central1-a" = "subnet-a" }
      nat_instance_name     = "test-prefix-nat"
      nat_instance_zone     = "ru-central1-a"
    }
  }

  override_module {
    target  = module.monitoring-service-account
    outputs = { apiKey = "exporter-from-secret" }
  }

  override_resource {
    target = yandex_vpc_network.kube
    values = { id = "network-created-b" }
  }

  assert {
    condition     = output.cloud_discovery_data.monitoringAPIKey == "exporter-from-secret"
    error_message = "expected the exporter key to be resolved from the credential Secret"
  }

  assert {
    condition     = output.cloud_discovery_data.natInstanceName == "test-prefix-nat"
    error_message = "expected the layout to plan the NAT instance from the ModuleConfig alone"
  }
}
