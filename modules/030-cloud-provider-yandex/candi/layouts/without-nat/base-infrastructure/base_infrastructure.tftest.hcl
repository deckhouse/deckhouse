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
# rejected-configuration scenarios live.

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

run "state_a_creates_network" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "WithoutNAT"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.20.0.0/16"
      zones           = ["ru-central1-a", "ru-central1-b"]
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
    condition     = output.cloud_discovery_data.defaultLbTargetGroupNetworkId == "network-created"
    error_message = "expected the created network to be used when no existing network is configured"
  }

  assert {
    condition     = jsonencode(output.cloud_discovery_data.zones) == jsonencode(["ru-central1-a", "ru-central1-b"])
    error_message = "expected the discovered zones to be intersected with the configured zones"
  }

  # WithoutNAT has no egress gateway, so nodes need their own public addresses.
  assert {
    condition     = output.cloud_discovery_data.shouldAssignPublicIPAddress == true
    error_message = "expected the WithoutNAT layout to require public addresses on nodes"
  }
}

run "state_a_reuses_existing_network" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion        = "deckhouse.io/v1"
      kind              = "YandexClusterConfiguration"
      layout            = "WithoutNAT"
      sshPublicKey      = "ssh-rsa AAAA"
      nodeNetworkCIDR   = "10.20.0.0/16"
      existingNetworkID = "network-existing"
      existingZoneToSubnetIDMap = {
        "ru-central1-a" = "subnet-a"
        "ru-central1-b" = "subnet-b"
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
      route_table_id = "route-table-a"
      zone_to_subnet_id_map = {
        "ru-central1-a" = "subnet-a"
        "ru-central1-b" = "subnet-b"
      }
      nat_instance_name = ""
      nat_instance_zone = ""
    }
  }

  assert {
    condition     = output.cloud_discovery_data.defaultLbTargetGroupNetworkId == "network-existing"
    error_message = "expected the existing network to be reused"
  }

  assert {
    condition     = jsonencode(output.cloud_discovery_data.zoneToSubnetIdMap) == jsonencode({ "ru-central1-a" = "subnet-a", "ru-central1-b" = "subnet-b" })
    error_message = "expected the existing subnets to be published"
  }
}

# State B: no YandexClusterConfiguration at all.
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
              layout          = "WithoutNAT"
              nodeNetworkCIDR = "10.70.0.0/16"
              dhcpOptions     = { domainName = "example.com" }
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
      nat_instance_name     = ""
      nat_instance_zone     = ""
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
    condition     = output.cloud_discovery_data.shouldAssignPublicIPAddress == true
    error_message = "expected the WithoutNAT layout to require public addresses on nodes"
  }
}

# vpc_components is left un-overridden here, unlike the runs above: the point is to
# exercise the real existing_route_table_id wiring from nodes.parameters through to
# vpc-components, not a canned output. If the layout failed to pass the value
# through, vpc-components would create its own yandex_vpc_route_table.kube and the
# mock provider would generate a fresh id for it, which would not equal the literal
# asserted below.
run "existing_route_table_id_is_honored" {
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
          provider = { parameters = { cloudID = "cloud-c", folderID = "folder-c" } }
          nodes = {
            parameters = {
              sshPublicKey         = "ssh-rsa STATE_C"
              layout               = "WithoutNAT"
              nodeNetworkCIDR      = "10.80.0.0/16"
              existingNetworkID    = "network-existing-c"
              existingRouteTableID = "route-table-existing-c"
            }
          }
        }
      }
    }
  }

  assert {
    condition     = output.cloud_discovery_data.routeTableID == "route-table-existing-c"
    error_message = "expected nodes.parameters.existingRouteTableID to reach vpc-components and be published as-is, proving no route table was created"
  }
}

# A dynamic zone in the global zone list flows through the layout: the
# discovery data includes ru-central1-f and excludes the omitted ru-central1-e.
run "state_b_dynamic_zone_discovery" {
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
              layout          = "WithoutNAT"
              nodeNetworkCIDR = "10.70.0.0/16"
              zones           = ["ru-central1-a", "ru-central1-b", "ru-central1-d", "ru-central1-f"]
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
        "ru-central1-f" = "subnet-f"
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
    condition     = jsonencode(output.cloud_discovery_data.zones) == jsonencode(["ru-central1-a", "ru-central1-b", "ru-central1-d", "ru-central1-f"])
    error_message = "expected the dynamic zone in the discovery data and the omitted ru-central1-e excluded"
  }

  assert {
    condition     = output.cloud_discovery_data.zoneToSubnetIdMap["ru-central1-f"] == "subnet-f" && !contains(keys(output.cloud_discovery_data.zoneToSubnetIdMap), "ru-central1-e")
    error_message = "expected the discovery subnet map to carry ru-central1-f without ru-central1-e"
  }
}

# An adopted map with a zone outside the static set and no explicit `zones`
# must keep the discovery zone list on the static zones (node-manager derives
# NodeGroup default zones from it), while the map itself keeps ru-central1-f.
run "adopted_map_without_zones_keeps_static_discovery" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "WithoutNAT"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.0.0.0/16"
      existingZoneToSubnetIDMap = {
        "ru-central1-a" = "subnet-a"
        "ru-central1-b" = "subnet-b"
        "ru-central1-f" = "subnet-f"
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
      route_table_id = "route-table-a"
      zone_to_subnet_id_map = {
        "ru-central1-a" = "subnet-a"
        "ru-central1-b" = "subnet-b"
        "ru-central1-f" = "subnet-f"
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
    condition     = jsonencode(output.cloud_discovery_data.zones) == jsonencode(["ru-central1-a", "ru-central1-b"])
    error_message = "expected the discovery zone list to stay on the static zones for an adopted map without explicit zones"
  }

  assert {
    condition     = output.cloud_discovery_data.zoneToSubnetIdMap["ru-central1-f"] == "subnet-f"
    error_message = "expected the adopted ru-central1-f subnet to stay in the subnet map"
  }
}

# State C regression: the migration is complete (ModuleConfig v2 plus the
# cluster resources below), yet a stale PCC Secret still lists zones. The
# stale list must not reach raw_zones: with PCC-first precedence the static
# discovery guard was defeated and ru-central1-f leaked into the discovery
# zone list.
run "state_c_stale_pcc_zones_keep_static_discovery" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "WithoutNAT"
      sshPublicKey    = "ssh-rsa STALE"
      nodeNetworkCIDR = "10.0.0.0/16"
      zones           = ["ru-central1-a", "ru-central1-b", "ru-central1-d"]
      existingZoneToSubnetIDMap = {
        "ru-central1-a" = "subnet-a"
        "ru-central1-b" = "subnet-b"
        "ru-central1-f" = "subnet-f"
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
              layout          = "WithoutNAT"
              nodeNetworkCIDR = "10.70.0.0/16"
              existingZoneToSubnetIDMap = {
                ru-central1-a = "subnet-a"
                ru-central1-b = "subnet-b"
                ru-central1-f = "subnet-f"
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
      route_table_id = "route-table-b"
      zone_to_subnet_id_map = {
        "ru-central1-a" = "subnet-a"
        "ru-central1-b" = "subnet-b"
        "ru-central1-f" = "subnet-f"
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
    condition     = jsonencode(output.cloud_discovery_data.zones) == jsonencode(["ru-central1-a", "ru-central1-b"])
    error_message = "expected the stale PCC zones to be ignored: an adopted map without explicit zones keeps the static discovery zones"
  }

  assert {
    condition     = output.cloud_discovery_data.zoneToSubnetIdMap["ru-central1-f"] == "subnet-f"
    error_message = "expected the adopted ru-central1-f subnet to stay in the subnet map"
  }
}
