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
# The Yandex Cloud provider is mocked, so nothing is created and no credentials
# are needed; the provider still has to be installable for its schema.
#
# vpc-components is overridden because its zone_to_subnet_id_map uses computed
# subnet ids as map keys, which stay unknown throughout a mocked plan.
#
# Rejected configurations are covered in
# terraform-modules/migration/migration.tftest.hcl: the checks are output
# preconditions of the migration module, and expect_failures cannot target a
# child module's output.

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

# State A: the layout still reads a YandexClusterConfiguration and creates its
# own network.
run "state_a_creates_network" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.0.0.0/16"
      zones           = ["ru-central1-a", "ru-central1-b"]
      labels          = { project = "cms" }
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8" }
      }
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  override_module {
    target = module.vpc_components
    outputs = {
      route_table_id = "route-table-a"
      zone_to_subnet_id_map = {
        "ru-central1-a" = "subnet-a"
        "ru-central1-b" = "subnet-b"
        "ru-central1-d" = "subnet-d"
      }
      nat_instance_name = ""
      nat_instance_zone = ""
    }
  }

  override_resource {
    target = yandex_vpc_network.kube
    values = { id = "network-created" }
  }

  assert {
    condition     = output.cloud_discovery_data.kind == "YandexCloudDiscoveryData"
    error_message = "expected YandexCloudDiscoveryData to be published"
  }

  assert {
    condition     = output.cloud_discovery_data.region == "ru-central1"
    error_message = "expected the ru-central1 region"
  }

  assert {
    condition     = output.cloud_discovery_data.defaultLbTargetGroupNetworkId == "network-created"
    error_message = "expected the created network to be used when no existing network is configured"
  }

  assert {
    condition     = output.cloud_discovery_data.routeTableID == "route-table-a"
    error_message = "expected the route table id from vpc-components"
  }

  # The discovery data must be narrowed to the zones the configuration allows.
  assert {
    condition     = jsonencode(output.cloud_discovery_data.zones) == jsonencode(["ru-central1-a", "ru-central1-b"])
    error_message = "expected the discovered zones to be intersected with the configured zones"
  }

  assert {
    condition     = output.cloud_discovery_data.shouldAssignPublicIPAddress == false
    error_message = "expected the Standard layout not to require public addresses on nodes"
  }
}

# State A with a pre-existing network: no yandex_vpc_network is created.
run "state_a_reuses_existing_network" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion        = "deckhouse.io/v1"
      kind              = "YandexClusterConfiguration"
      layout            = "Standard"
      sshPublicKey      = "ssh-rsa AAAA"
      nodeNetworkCIDR   = "10.0.0.0/16"
      existingNetworkID = "network-existing"
      existingZoneToSubnetIDMap = {
        "ru-central1-a" = "subnet-a"
      }
      dhcpOptions = {
        domainName        = "example.com"
        domainNameServers = ["10.0.0.2"]
      }
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8" }
      }
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  override_module {
    target = module.vpc_components
    outputs = {
      route_table_id        = "route-table-a"
      zone_to_subnet_id_map = { "ru-central1-a" = "subnet-a" }
      nat_instance_name     = ""
      nat_instance_zone     = ""
    }
  }

  assert {
    condition     = output.cloud_discovery_data.defaultLbTargetGroupNetworkId == "network-existing"
    error_message = "expected the existing network to be reused"
  }

  assert {
    condition     = jsonencode(output.cloud_discovery_data.internalNetworkIDs) == jsonencode(["network-existing"])
    error_message = "expected the existing network to be published as an internal network"
  }

  # No cluster-wide zones: every zone the subnets cover is discovered.
  assert {
    condition     = jsonencode(output.cloud_discovery_data.zones) == jsonencode(["ru-central1-a"])
    error_message = "expected all subnet zones to be discovered when the configuration restricts none"
  }
}

# State B: no YandexClusterConfiguration at all. This is the case that used to
# crash with "lookup: argument must not be null".
run "state_b_module_config_only" {
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
        spec       = { cores = 4, memory = 8192, imageID = "fd8", platformID = "standard-v2", diskType = "network-ssd" }
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
              layout          = "Standard"
              nodeNetworkCIDR = "10.70.0.0/16"
              zones           = ["ru-central1-a", "ru-central1-d"]
              labels          = { env = "prod" }
            }
          }
        }
      }
    }
  }

  override_module {
    target = module.vpc_components
    outputs = {
      route_table_id = "route-table-b"
      zone_to_subnet_id_map = {
        "ru-central1-a" = "subnet-a"
        "ru-central1-b" = "subnet-b"
        "ru-central1-d" = "subnet-d"
      }
      nat_instance_name = ""
      nat_instance_zone = ""
    }
  }

  override_resource {
    target = yandex_vpc_network.kube
    values = { id = "network-created-b" }
  }

  assert {
    condition     = output.cloud_discovery_data.routeTableID == "route-table-b"
    error_message = "expected the layout to plan from the ModuleConfig alone"
  }

  assert {
    condition     = jsonencode(output.cloud_discovery_data.zones) == jsonencode(["ru-central1-a", "ru-central1-d"])
    error_message = "expected the zones to come from the ModuleConfig"
  }
}
