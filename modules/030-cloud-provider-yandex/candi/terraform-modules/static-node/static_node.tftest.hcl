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

# Run from this directory with OpenTofu >= 1.12 — the version pinned in
# candi/terraform_versions.yml:
#
#   tofu version                          # must report v1.12.0 or newer
#   tofu init -backend=false && tofu test
#
# The override targets below are resource addresses without an index on purpose.
# An instance address such as data.yandex_vpc_subnet.kube_a[0] is rejected outright
# ("Resource instance address with keys is not allowed", verified on v1.12.5), and a
# non-indexed target overrides every instance of a count-ed resource, which is what
# these suites need. A .tftest.hcl file may not contain a terraform{} block, so the
# version cannot be enforced by a required_version constraint — check it by hand.
#
# This module is a root module: layouts/<layout>/static-node symlinks it. The
# Yandex Cloud provider is mocked, so nothing is created and no credentials are
# needed; the provider still has to be installable for its schema.
#
# The module publishes no outputs, so producing a plan at all is the assertion.
# That is what these tests guard: the module used to fail outright on a
# ModuleConfig-only cluster because it read the YandexClusterConfiguration
# directly.
#
# Rejected configurations that the migration module owns are covered in
# terraform-modules/migration/migration.tftest.hcl: those checks are output
# preconditions of the migration module, and expect_failures cannot target a
# child module's output. The networkType check below lives in this module, so it
# can be asserted here.

mock_provider "yandex" {}

variables {
  clusterConfiguration         = { cloud = { prefix = "test-prefix" } }
  clusterUUID                  = "11111111-1111-1111-1111-111111111111"
  nodeGroupName                = "worker"
  nodeIndex                    = 0
  cloudConfig                  = ""
  providerClusterConfiguration = null
  nodeGroups                   = {}
  instanceClasses              = {}
  secrets                      = {}
  settings                     = null
}

run "state_a_worker" {
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
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8", etcdDiskSizeGb = 20 }
      }
      nodeGroups = [
        {
          name     = "worker"
          replicas = 2
          instanceClass = {
            cores            = 4
            memory           = 4096
            imageID          = "fd8worker"
            coreFraction     = 20
            additionalLabels = { role = "worker" }
          }
        },
      ]
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  override_data {
    target = data.yandex_vpc_subnet.kube_a
    values = { id = "subnet-a", zone = "ru-central1-a" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_b
    values = { id = "subnet-b", zone = "ru-central1-b" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_e
    values = { id = "subnet-e", zone = "ru-central1-e" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_d
    values = { id = "subnet-d", zone = "ru-central1-d" }
  }
}

# A node group restricted to a single zone, with reserved addresses and an extra
# network interface.
run "state_a_worker_with_external_addressing" {
  command = plan

  variables {
    nodeIndex = 1

    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "WithoutNAT"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.0.0.0/16"
      zones           = ["ru-central1-a", "ru-central1-b"]
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8", etcdDiskSizeGb = 20 }
      }
      nodeGroups = [
        {
          name     = "worker"
          replicas = 2
          zones    = ["ru-central1-b"]
          instanceClass = {
            cores               = 4
            memory              = 4096
            imageID             = "fd8worker"
            networkType         = "SoftwareAccelerated"
            externalSubnetID    = "subnet-main"
            externalSubnetIDs   = ["subnet-ext-a", "subnet-ext-b"]
            externalIPAddresses = ["203.0.113.1", "Auto"]
          }
        },
      ]
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  override_data {
    target = data.yandex_vpc_subnet.kube_a
    values = { id = "subnet-a", zone = "ru-central1-a" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_b
    values = { id = "subnet-b", zone = "ru-central1-b" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_e
    values = { id = "subnet-e", zone = "ru-central1-e" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_d
    values = { id = "subnet-d", zone = "ru-central1-d" }
  }
}

# State B: no YandexClusterConfiguration, and the node group is selected by name
# out of the cluster resources.
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
      worker = {
        apiVersion = "deckhouse.io/v1"
        kind       = "NodeGroup"
        metadata   = { name = "worker" }
        spec = {
          nodeType = "CloudPermanent"
          cloudInstances = {
            classReference = { kind = "YandexInstanceClass", name = "worker-87eba76e7f31" }
            minPerZone     = 2
            maxPerZone     = 2
            zones          = ["ru-central1-b"]
          }
        }
      }
    }

    instanceClasses = {
      "master-fc613b4dfd67" = {
        apiVersion = "deckhouse.io/v1"
        kind       = "YandexInstanceClass"
        metadata   = { name = "master-fc613b4dfd67" }
        spec       = { cores = 4, memory = 8192, imageID = "fd8", etcdDiskSizeGB = 20 }
      }
      "worker-87eba76e7f31" = {
        apiVersion = "deckhouse.io/v1"
        kind       = "YandexInstanceClass"
        metadata   = { name = "worker-87eba76e7f31" }
        spec = {
          cores                 = 4
          memory                = 4096
          imageID               = "fd8worker"
          platformID            = "standard-v3"
          diskType              = "network-hdd"
          diskSizeGB            = 40
          coreFraction          = 20
          networkType           = "Standard"
          assignPublicIPAddress = true
          additionalLabels      = { role = "worker" }
          additionalSubnets     = ["subnet-ext-b"]
        }
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
              zones           = ["ru-central1-a", "ru-central1-b"]
              labels          = { env = "prod" }
            }
          }
        }
      }
    }
  }

  override_data {
    target = data.yandex_vpc_subnet.kube_a
    values = { id = "subnet-a", zone = "ru-central1-a" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_b
    values = { id = "subnet-b", zone = "ru-central1-b" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_e
    values = { id = "subnet-e", zone = "ru-central1-e" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_d
    values = { id = "subnet-d", zone = "ru-central1-d" }
  }
}

# State B where the ModuleConfig supplies the per-node-group external subnets and
# addresses instead of the instance class.
run "state_b_with_external_addressing_from_module_config" {
  command = plan

  variables {
    nodeGroups = {
      worker = {
        apiVersion = "deckhouse.io/v1"
        kind       = "NodeGroup"
        metadata   = { name = "worker" }
        spec = {
          nodeType = "CloudPermanent"
          cloudInstances = {
            classReference = { kind = "YandexInstanceClass", name = "worker-87eba76e7f31" }
            minPerZone     = 1
            maxPerZone     = 1
          }
        }
      }
    }

    instanceClasses = {
      "worker-87eba76e7f31" = {
        apiVersion = "deckhouse.io/v1"
        kind       = "YandexInstanceClass"
        metadata   = { name = "worker-87eba76e7f31" }
        spec       = { cores = 2, memory = 2048, imageID = "fd8worker" }
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
              sshPublicKey        = "ssh-rsa STATE_B"
              layout              = "WithoutNAT"
              nodeNetworkCIDR     = "10.70.0.0/16"
              externalIPAddresses = { worker = ["Auto"] }
              externalSubnetIDs   = { worker = ["subnet-ext-a"] }
            }
          }
        }
      }
    }
  }

  override_data {
    target = data.yandex_vpc_subnet.kube_a
    values = { id = "subnet-a", zone = "ru-central1-a" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_b
    values = { id = "subnet-b", zone = "ru-central1-b" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_e
    values = { id = "subnet-e", zone = "ru-central1-e" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_d
    values = { id = "subnet-d", zone = "ru-central1-d" }
  }

  # The instance class above omits platformID and diskType, exactly like the copy
  # dhctl parses out of the bootstrap resources YAML before the apiserver applies
  # the CRD defaults. The fallbacks must be the CRD defaults, otherwise the first
  # converge after bootstrap replaces the node.
  assert {
    condition     = yandex_compute_instance.static.platform_id == "standard-v3"
    error_message = "expected an omitted platformID to fall back to the CRD default standard-v3"
  }

  assert {
    condition     = yandex_compute_instance.static.boot_disk[0].initialize_params[0].type == "network-hdd"
    error_message = "expected an omitted diskType to fall back to the CRD default network-hdd"
  }
}

# A v1alpha1 YandexInstanceClass reaches this module unconverted when dhctl parses
# the bootstrap resources YAML, so the SCREAMING_SNAKE_CASE networkType must map to
# the same provider value as the v1 spelling instead of falling back to null.
run "state_b_v1alpha1_network_type_is_mapped" {
  command = plan

  variables {
    nodeGroups = {
      worker = {
        apiVersion = "deckhouse.io/v1"
        kind       = "NodeGroup"
        metadata   = { name = "worker" }
        spec = {
          nodeType = "CloudPermanent"
          cloudInstances = {
            classReference = { kind = "YandexInstanceClass", name = "worker-87eba76e7f31" }
            minPerZone     = 1
            maxPerZone     = 1
          }
        }
      }
    }

    instanceClasses = {
      "worker-87eba76e7f31" = {
        apiVersion = "deckhouse.io/v1alpha1"
        kind       = "YandexInstanceClass"
        metadata   = { name = "worker-87eba76e7f31" }
        spec = {
          cores       = 4
          memory      = 4096
          imageID     = "fd8worker"
          networkType = "SOFTWARE_ACCELERATED"
        }
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
            }
          }
        }
      }
    }
  }

  override_data {
    target = data.yandex_vpc_subnet.kube_a
    values = { id = "subnet-a", zone = "ru-central1-a" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_b
    values = { id = "subnet-b", zone = "ru-central1-b" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_e
    values = { id = "subnet-e", zone = "ru-central1-e" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_d
    values = { id = "subnet-d", zone = "ru-central1-d" }
  }

  assert {
    condition     = yandex_compute_instance.static.network_acceleration_type == "software_accelerated"
    error_message = "expected the v1alpha1 SOFTWARE_ACCELERATED spelling to map to software_accelerated"
  }
}

# An unset networkType keeps the provider default, matching the pre-migration
# behaviour of YandexClusterConfiguration without networkType.
run "state_b_unset_network_type_keeps_provider_default" {
  command = plan

  variables {
    nodeGroups = {
      worker = {
        apiVersion = "deckhouse.io/v1"
        kind       = "NodeGroup"
        metadata   = { name = "worker" }
        spec = {
          nodeType = "CloudPermanent"
          cloudInstances = {
            classReference = { kind = "YandexInstanceClass", name = "worker-87eba76e7f31" }
            minPerZone     = 1
            maxPerZone     = 1
          }
        }
      }
    }

    instanceClasses = {
      "worker-87eba76e7f31" = {
        apiVersion = "deckhouse.io/v1"
        kind       = "YandexInstanceClass"
        metadata   = { name = "worker-87eba76e7f31" }
        spec       = { cores = 4, memory = 4096, imageID = "fd8worker" }
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
            }
          }
        }
      }
    }
  }

  override_data {
    target = data.yandex_vpc_subnet.kube_a
    values = { id = "subnet-a", zone = "ru-central1-a" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_b
    values = { id = "subnet-b", zone = "ru-central1-b" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_e
    values = { id = "subnet-e", zone = "ru-central1-e" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_d
    values = { id = "subnet-d", zone = "ru-central1-d" }
  }

  assert {
    condition     = yandex_compute_instance.static.network_acceleration_type == null
    error_message = "expected an unset networkType to leave network_acceleration_type at the provider default"
  }
}

# A networkType this module cannot translate must fail the plan rather than
# silently degrade to the provider default.
run "state_b_unknown_network_type_is_rejected" {
  command = plan

  variables {
    nodeGroups = {
      worker = {
        apiVersion = "deckhouse.io/v1"
        kind       = "NodeGroup"
        metadata   = { name = "worker" }
        spec = {
          nodeType = "CloudPermanent"
          cloudInstances = {
            classReference = { kind = "YandexInstanceClass", name = "worker-87eba76e7f31" }
            minPerZone     = 1
            maxPerZone     = 1
          }
        }
      }
    }

    instanceClasses = {
      "worker-87eba76e7f31" = {
        apiVersion = "deckhouse.io/v1"
        kind       = "YandexInstanceClass"
        metadata   = { name = "worker-87eba76e7f31" }
        spec = {
          cores       = 4
          memory      = 4096
          imageID     = "fd8worker"
          networkType = "software_accelerated"
        }
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
            }
          }
        }
      }
    }
  }

  override_data {
    target = data.yandex_vpc_subnet.kube_a
    values = { id = "subnet-a", zone = "ru-central1-a" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_b
    values = { id = "subnet-b", zone = "ru-central1-b" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_e
    values = { id = "subnet-e", zone = "ru-central1-e" }
  }
  override_data {
    target = data.yandex_vpc_subnet.kube_d
    values = { id = "subnet-d", zone = "ru-central1-d" }
  }

  expect_failures = [yandex_compute_instance.static]
}
