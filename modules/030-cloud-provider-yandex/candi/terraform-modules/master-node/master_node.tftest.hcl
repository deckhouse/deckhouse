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
# This module is a root module: layouts/<layout>/master-node symlinks it. The
# Yandex Cloud provider is mocked, so nothing is created and no credentials are
# needed; the provider still has to be installable for its schema.
#
# No module under candi/ declares required_providers: dhctl generates and injects a versions.tf
# at runtime. A bare `tofu init` here therefore resolves the *implied* address hashicorp/yandex
# (latest 0.114.0), which is the wrong namespace and far behind the pinned
# yandex-cloud/yandex 0.174.0 - its plugin fails to answer, and the runs are validated against
# the wrong schema. Drop in the same versions.tf dhctl injects before running the suite:
#
#   terraform {
#     required_providers {
#       yandex = { source = "yandex-cloud/yandex", version = "0.174.0" }
#     }
#   }
#
# Keep it out of the committed tree so the runtime injection is not perturbed.
#
# Every output of this module is a compute instance attribute and therefore
# unknown during a mocked plan, so a run that produces a plan at all is the
# assertion. That is exactly the regression these tests guard: the module used to
# fail outright on a ModuleConfig-only cluster, and to reference an etcd disk size
# that no longer existed.
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
  nodeIndex                    = 0
  cloudConfig                  = ""
  providerClusterConfiguration = null
  nodeGroups                   = {}
  instanceClasses              = {}
  secrets                      = {}
  settings                     = null
}

# State A with the deckhouse-created subnets.
run "state_a_standard" {
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
        replicas = 3
        instanceClass = {
          cores            = 4
          memory           = 8192
          imageID          = "fd8"
          etcdDiskSizeGb   = 20
          additionalLabels = { role = "master" }
        }
      }
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

  assert {
    condition     = output.kubernetes_data_device_path == "/dev/disk/by-id/virtio-kubernetes-data"
    error_message = "expected a stable device path for the kubernetes-data disk"
  }

  # The PCC above omits platform, diskType and diskSizeGB, so the migration projection has to
  # supply the *pre-migration terraform* values. Getting these wrong replaces the master boot
  # disk and the etcd disk of every cluster that never set them, so assert the configured
  # attributes rather than only that a plan is produced: a typo in the read path
  # (e.g. platformId for platformID) would otherwise fall through try() to the v1 fallbacks
  # standard-v3 / network-hdd and go unnoticed.
  assert {
    condition     = yandex_compute_instance.master.platform_id == "standard-v2"
    error_message = "expected the PCC projection to pin the pre-migration platform standard-v2"
  }

  assert {
    condition     = yandex_compute_disk.kubernetes_data.type == "network-ssd"
    error_message = "expected the PCC projection to pin the pre-migration disk type network-ssd"
  }

  # etcdDiskSizeGb: 20 in the PCC, spelled with a lowercase b, must reach the CRD-spelled
  # etcdDiskSizeGB and size the etcd disk.
  assert {
    condition     = yandex_compute_disk.kubernetes_data.size == 20
    error_message = "expected etcdDiskSizeGb: 20 from the PCC to size the kubernetes-data disk"
  }

  # nodeIndex defaults to 0, so the node lands in the first cluster zone. The disk zone is
  # ForceNew: a change in zone ordering recreates the etcd disk.
  assert {
    condition     = yandex_compute_disk.kubernetes_data.zone == "ru-central1-a"
    error_message = "expected node 0 to take the first cluster zone, ru-central1-a"
  }

  # diskSizeGB is omitted above, so the projection's default of 50 has to reach the boot disk.
  assert {
    condition     = yandex_compute_instance.master.boot_disk[0].initialize_params[0].size == 50
    error_message = "expected an omitted diskSizeGB to become the pre-migration default of 50"
  }

  assert {
    condition     = yandex_compute_instance.master.boot_disk[0].initialize_params[0].type == "network-ssd"
    error_message = "expected the boot disk to use the pre-migration disk type network-ssd"
  }
}

# State A with reserved external addresses and an extra network interface: the
# deprecated externalSubnetID plus externalSubnetIDs plus externalIPAddresses.
run "state_a_with_external_addressing" {
  command = plan

  variables {
    nodeIndex = 1

    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.0.0.0/16"
      masterNodeGroup = {
        replicas = 3
        instanceClass = {
          cores               = 4
          memory              = 8192
          imageID             = "fd8"
          etcdDiskSizeGb      = 20
          networkType         = "SoftwareAccelerated"
          externalIPAddresses = ["203.0.113.1", "203.0.113.2", "Auto"]
          externalSubnetIDs   = ["subnet-ext-a", "subnet-ext-b", "subnet-ext-d"]
        }
      }
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

  assert {
    condition     = output.kubernetes_data_device_path == "/dev/disk/by-id/virtio-kubernetes-data"
    error_message = "expected a stable device path for the kubernetes-data disk"
  }
}

# State A using pre-existing subnets: the zone-to-subnet map drives a for_each
# data source instead of the four by-name lookups.
run "state_a_with_existing_subnets" {
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
      }
      masterNodeGroup = {
        replicas = 1
        zones    = ["ru-central1-b"]
        instanceClass = {
          cores          = 4
          memory         = 8192
          imageID        = "fd8"
          etcdDiskSizeGb = 20
        }
      }
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  # override_data addresses a whole data source, never a single instance, so the
  # for_each'd `existing` lookup gets one override shared by both zones. The run
  # only has to produce a plan, and masterNodeGroup.zones pins the node to
  # ru-central1-b, so the ru-central1-b values are the ones worth supplying.
  override_data {
    target = data.yandex_vpc_subnet.existing
    values = { id = "subnet-b", zone = "ru-central1-b" }
  }

  assert {
    condition     = output.kubernetes_data_device_path == "/dev/disk/by-id/virtio-kubernetes-data"
    error_message = "expected a stable device path for the kubernetes-data disk"
  }
}

# State B: no YandexClusterConfiguration. The compute parameters, the etcd disk
# size and the external addressing all come from the new resources.
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
            minPerZone     = 3
            maxPerZone     = 3
            zones          = ["ru-central1-a", "ru-central1-b"]
          }
        }
      }
    }

    instanceClasses = {
      "master-fc613b4dfd67" = {
        apiVersion = "deckhouse.io/v1"
        kind       = "YandexInstanceClass"
        metadata   = { name = "master-fc613b4dfd67" }
        spec = {
          cores                 = 4
          memory                = 8192
          imageID               = "fd8"
          platformID            = "standard-v2"
          diskType              = "network-ssd"
          diskSizeGB            = 50
          coreFraction          = 100
          networkType           = "Standard"
          assignPublicIPAddress = false
          etcdDiskSizeGB        = 20
          additionalLabels      = { role = "master" }
          mainSubnet            = "subnet-main"
          additionalSubnets     = ["subnet-ext-a"]
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

  assert {
    condition     = output.kubernetes_data_device_path == "/dev/disk/by-id/virtio-kubernetes-data"
    error_message = "expected a stable device path for the kubernetes-data disk"
  }
}

# State B where the ModuleConfig asks for automatically allocated public
# addresses through nodes.parameters instead of the instance class boolean.
run "state_b_with_external_ip_addresses_from_module_config" {
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
        spec = {
          cores          = 4
          memory         = 8192
          imageID        = "fd8"
          etcdDiskSizeGB = 20
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
              sshPublicKey        = "ssh-rsa STATE_B"
              layout              = "Standard"
              nodeNetworkCIDR     = "10.70.0.0/16"
              externalIPAddresses = { master = ["Auto"] }
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
    condition     = output.kubernetes_data_device_path == "/dev/disk/by-id/virtio-kubernetes-data"
    error_message = "expected a stable device path for the kubernetes-data disk"
  }

  # The instance class above omits platformID and diskType. The CRD carries no real `default:`
  # for them - only x-doc-default - so neither an instance class read back from the cluster nor
  # the copy dhctl parses out of the bootstrap resources YAML arrives with them filled in. These
  # fallbacks are the single source of the values, and they must match what the CRD documents
  # for v1, otherwise bootstrap and the first converge disagree and replace the master node and
  # its etcd disk.
  assert {
    condition     = yandex_compute_instance.master.platform_id == "standard-v3"
    error_message = "expected an omitted platformID to fall back to the documented v1 value standard-v3"
  }

  assert {
    condition     = yandex_compute_disk.kubernetes_data.type == "network-hdd"
    error_message = "expected an omitted diskType to fall back to the documented v1 value network-hdd"
  }
}

# A v1alpha1 YandexInstanceClass reaches this module unconverted when dhctl parses
# the bootstrap resources YAML, so the SCREAMING_SNAKE_CASE networkType must map to
# the same provider value as the v1 spelling instead of falling back to null.
run "state_b_v1alpha1_network_type_is_mapped" {
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
        apiVersion = "deckhouse.io/v1alpha1"
        kind       = "YandexInstanceClass"
        metadata   = { name = "master-fc613b4dfd67" }
        spec = {
          cores          = 4
          memory         = 8192
          imageID        = "fd8"
          etcdDiskSizeGB = 20
          networkType    = "SOFTWARE_ACCELERATED"
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
    condition     = yandex_compute_instance.master.network_acceleration_type == "software_accelerated"
    error_message = "expected the v1alpha1 SOFTWARE_ACCELERATED spelling to map to software_accelerated"
  }
}

# An unset networkType keeps the provider default, matching the pre-migration
# behaviour of YandexClusterConfiguration without networkType.
run "state_b_unset_network_type_keeps_provider_default" {
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
        spec = {
          cores          = 4
          memory         = 8192
          imageID        = "fd8"
          etcdDiskSizeGB = 20
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
    condition     = yandex_compute_instance.master.network_acceleration_type == null
    error_message = "expected an unset networkType to leave network_acceleration_type at the provider default"
  }
}

# A networkType this module cannot translate must fail the plan rather than
# silently degrade to the provider default.
run "state_b_unknown_network_type_is_rejected" {
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
        spec = {
          cores          = 4
          memory         = 8192
          imageID        = "fd8"
          etcdDiskSizeGB = 20
          networkType    = "SoftwareAcelerated"
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

  expect_failures = [yandex_compute_instance.master]
}
