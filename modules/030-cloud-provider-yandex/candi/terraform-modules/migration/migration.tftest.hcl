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

# Run with: tofu init -backend=false && tofu test
#
# Scenario naming:
#   state_a_*    YandexClusterConfiguration is the source of truth
#   state_b_*    cluster resources are the source of truth
#   transition_* both sources present, migration not finished
#   negative_*   configuration that must be rejected
#
# Instance class names are the SHA-256 suffixes produced by
# go_lib/cloud-provider/api.BuildInstanceClassName: master-fc613b4dfd67,
# worker-87eba76e7f31, system-bbc5e661e106.

variables {
  providerClusterConfiguration = null
  nodeGroups                   = {}
  instanceClasses              = {}
  secrets                      = {}
  settings                     = null
}

# --- State A: YandexClusterConfiguration ------------------------------------

# The Standard layout with a minimal instanceClass. Guards the defaults: a PCC
# that omits platform/diskType/diskSizeGB/etcdDiskSizeGb must keep producing the
# values the pre-migration terraform used, otherwise migrating an existing
# cluster replaces master disks and platforms.
run "state_a_standard_minimal" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC"
      nodeNetworkCIDR = "10.0.0.0/16"
      zones           = ["ru-central1-a", "ru-central1-b", "ru-central1-d"]
      masterNodeGroup = {
        replicas = 3
        instanceClass = {
          cores   = 4
          memory  = 8192
          imageID = "fd8v9q2qspfnsv055rh"
        }
      }
      provider = {
        cloudID            = "cloud-test-123"
        folderID           = "folder-test-456"
        serviceAccountJSON = "{\"id\":\"sa-test\"}"
      }
    }
  }

  assert {
    condition     = output.settings.kind == "ModuleConfig" && output.settings.spec.settings.provider.parameters.cloudID == "cloud-test-123"
    error_message = "expected the PCC to be projected onto a ModuleConfig while no new resources exist"
  }

  assert {
    condition     = output.settings.spec.version == 2
    error_message = "expected the synthesised ModuleConfig to be version 2"
  }

  assert {
    condition     = output.settings.spec.enabled
    error_message = "expected the synthesised ModuleConfig to be enabled"
  }

  assert {
    condition     = output.settings.metadata.name == "cloud-provider-yandex"
    error_message = "expected the synthesised ModuleConfig to be named cloud-provider-yandex"
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.cloudID == "cloud-test-123" && output.settings.spec.settings.provider.parameters.folderID == "folder-test-456"
    error_message = "expected provider coordinates to come from the PCC provider block"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.layout == "Standard"
    error_message = "expected layout Standard"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.nodeNetworkCIDR == "10.0.0.0/16"
    error_message = "expected nodeNetworkCIDR to come from the PCC"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.sshPublicKey == "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC"
    error_message = "expected sshPublicKey to come from the PCC"
  }

  assert {
    condition     = jsonencode(output.settings.spec.settings.nodes.parameters.zones) == jsonencode(["ru-central1-a", "ru-central1-b", "ru-central1-d"])
    error_message = "expected cluster-wide zones to come from the PCC"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.existingNetworkID == "" && length(output.settings.spec.settings.nodes.parameters.existingZoneToSubnetIDMap) == 0
    error_message = "expected no existing network to be referenced"
  }

  assert {
    condition = (
      output.settings.spec.settings.nodes.parameters.dhcpOptions.domainName == "" &&
      length(output.settings.spec.settings.nodes.parameters.dhcpOptions.domainNameServers) == 0
    )
    error_message = "expected absent DHCP options to project to zero values"
  }

  assert {
    condition = (
      output.settings.spec.settings.nodes.parameters.withNATInstance.internalSubnetID == "" &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.externalSubnetID == "" &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.internalSubnetCIDR == "" &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.natInstanceExternalAddress == "" &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.natInstanceInternalAddress == ""
    )
    error_message = "expected an absent withNATInstance block to project to zero values"
  }

  assert {
    condition = (
      output.settings.spec.settings.nodes.parameters.withNATInstance.natInstanceResources.cores == 2 &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.natInstanceResources.memory == 2048 &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.natInstanceResources.platform == "standard-v2"
    )
    error_message = "expected default NAT instance sizing"
  }

  assert {
    condition     = jsonencode(keys(output.nodeGroups)) == jsonencode(["master"])
    error_message = "expected exactly one synthesised NodeGroup for a master-only PCC"
  }

  assert {
    condition     = output.nodeGroups.master.spec.nodeType == "CloudPermanent"
    error_message = "expected the master NodeGroup to be CloudPermanent"
  }

  assert {
    condition = (
      output.nodeGroups.master.spec.cloudInstances.minPerZone == 3 &&
      output.nodeGroups.master.spec.cloudInstances.maxPerZone == 3
    )
    error_message = "expected minPerZone and maxPerZone to mirror masterNodeGroup.replicas"
  }

  assert {
    condition     = output.nodeGroups.master.spec.cloudInstances.classReference.kind == "YandexInstanceClass"
    error_message = "expected a YandexInstanceClass class reference"
  }

  assert {
    condition     = output.nodeGroups.master.spec.cloudInstances.classReference.name == "master-fc613b4dfd67"
    error_message = "expected the hashed instance class name produced by BuildInstanceClassName"
  }

  assert {
    condition     = jsonencode(output.nodeGroups.master.spec.cloudInstances.zones) == jsonencode(["ru-central1-a", "ru-central1-b", "ru-central1-d"])
    error_message = "expected the master NodeGroup to inherit the cluster-wide zones"
  }

  assert {
    condition = (
      output.nodeGroups.master.spec.nodeTemplate.labels["node-role.kubernetes.io/control-plane"] == "" &&
      output.nodeGroups.master.spec.nodeTemplate.labels["node-role.kubernetes.io/master"] == ""
    )
    error_message = "expected control-plane labels on the master NodeGroup"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].metadata.name == "master-fc613b4dfd67"
    error_message = "expected the instance class metadata.name to match the map key"
  }

  assert {
    condition = (
      output.instanceClasses["master-fc613b4dfd67"].spec.cores == 4 &&
      output.instanceClasses["master-fc613b4dfd67"].spec.memory == 8192 &&
      output.instanceClasses["master-fc613b4dfd67"].spec.imageID == "fd8v9q2qspfnsv055rh"
    )
    error_message = "expected the compute parameters to come from the PCC instanceClass"
  }

  # Regression guard: the YandexInstanceClass CRD defaults platformID to
  # standard-v3 and diskType to network-hdd. Using those here would replace the
  # masters of every cluster that never set them.
  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.platformID == "standard-v2"
    error_message = "expected an omitted PCC platform to default to standard-v2, not to the CRD default"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.diskType == "network-ssd"
    error_message = "expected an omitted PCC diskType to default to network-ssd, not to the CRD default"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.diskSizeGB == 50
    error_message = "expected an omitted PCC diskSizeGB to default to 50"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.etcdDiskSizeGB == 10
    error_message = "expected an omitted PCC etcdDiskSizeGb to default to 10"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.networkType == "Standard"
    error_message = "expected an omitted PCC networkType to default to Standard"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.mainSubnet == ""
    error_message = "expected no main subnet when the PCC omits externalSubnetID"
  }

  assert {
    condition     = length(output.instanceClasses["master-fc613b4dfd67"].spec.additionalSubnets) == 0
    error_message = "expected no additional subnets when the PCC omits externalSubnetIDs"
  }

  assert {
    condition     = length(output.settings.spec.settings.nodes.parameters.externalIPAddresses) == 0 && length(output.settings.spec.settings.nodes.parameters.externalSubnetIDs) == 0
    error_message = "expected no external addressing when the PCC omits it"
  }

  # dhctl keys credential Secrets by "<namespace>/<name>"; the synthesised map
  # must use the same shape so both states look identical to consumers.
  assert {
    condition     = jsonencode(keys(nonsensitive(output.secrets))) == jsonencode(["d8-cloud-provider-yandex/d8-credentials"])
    error_message = "expected a single credential Secret keyed by namespace/name"
  }

  assert {
    condition = (
      nonsensitive(output.secrets)["d8-cloud-provider-yandex/d8-credentials"].type == "cloud-provider.deckhouse.io/credentials" &&
      nonsensitive(output.secrets)["d8-cloud-provider-yandex/d8-credentials"].stringData.authScheme == "serviceAccount"
    )
    error_message = "expected a serviceAccount credential Secret"
  }

  assert {
    condition     = nonsensitive(output.secrets)["d8-cloud-provider-yandex/d8-credentials"].stringData.secret == "{\"id\":\"sa-test\"}"
    error_message = "expected the service account key to be taken from the PCC"
  }
}

# Every instanceClass field set explicitly: nothing may be overwritten by a
# default, and the deprecated externalSubnetID / externalSubnetIDs /
# externalIPAddresses trio must land where the node modules look for it.
run "state_a_standard_explicit_instance_class" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.10.0.0/16"
      labels          = { project = "cms" }
      masterNodeGroup = {
        replicas = 1
        instanceClass = {
          cores               = 8
          memory              = 16384
          imageID             = "fd8master"
          platform            = "standard-v3"
          diskType            = "network-ssd-nonreplicated"
          diskSizeGB          = 93
          etcdDiskSizeGb      = 20
          networkType         = "SoftwareAccelerated"
          additionalLabels    = { role = "master" }
          externalSubnetID    = "subnet-main"
          externalSubnetIDs   = ["subnet-ext-a", "subnet-ext-b"]
          externalIPAddresses = ["1.2.3.4", "Auto"]
        }
      }
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  assert {
    condition = (
      output.instanceClasses["master-fc613b4dfd67"].spec.platformID == "standard-v3" &&
      output.instanceClasses["master-fc613b4dfd67"].spec.diskType == "network-ssd-nonreplicated" &&
      output.instanceClasses["master-fc613b4dfd67"].spec.diskSizeGB == 93 &&
      output.instanceClasses["master-fc613b4dfd67"].spec.etcdDiskSizeGB == 20 &&
      output.instanceClasses["master-fc613b4dfd67"].spec.networkType == "SoftwareAccelerated"
    )
    error_message = "expected explicit PCC instanceClass values to be preserved"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.mainSubnet == "subnet-main"
    error_message = "expected the deprecated externalSubnetID to become mainSubnet"
  }

  assert {
    condition     = jsonencode(output.instanceClasses["master-fc613b4dfd67"].spec.additionalSubnets) == jsonencode(["subnet-ext-a", "subnet-ext-b"])
    error_message = "expected externalSubnetIDs to become additionalSubnets"
  }

  assert {
    condition     = jsonencode(output.instanceClasses["master-fc613b4dfd67"].spec.additionalLabels) == jsonencode({ role = "master" })
    error_message = "expected additionalLabels to be preserved on the instance class"
  }

  # YandexInstanceClass has no externalIPAddresses counterpart, so the list is
  # keyed by node group name in nodes.parameters instead.
  assert {
    condition     = jsonencode(output.settings.spec.settings.nodes.parameters.externalIPAddresses["master"]) == jsonencode(["1.2.3.4", "Auto"])
    error_message = "expected externalIPAddresses to be keyed by node group name"
  }

  assert {
    condition     = jsonencode(output.settings.spec.settings.nodes.parameters.externalSubnetIDs["master"]) == jsonencode(["subnet-ext-a", "subnet-ext-b"])
    error_message = "expected externalSubnetIDs to also be keyed by node group name"
  }

  assert {
    condition     = jsonencode(output.settings.spec.settings.nodes.parameters.labels) == jsonencode({ project = "cms" })
    error_message = "expected cluster-wide labels to be preserved"
  }

  # No cluster-wide zones and no node group zones: consumers must fall back to
  # every zone the subnets cover.
  assert {
    condition     = output.nodeGroups.master.spec.cloudInstances.zones == null
    error_message = "expected no explicit zones when neither the PCC nor the node group sets them"
  }
}

# WithoutNAT relies on pre-existing networks and DHCP options.
run "state_a_without_nat" {
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
      dhcpOptions = {
        domainName        = "example.com"
        domainNameServers = ["10.20.0.2", "10.20.0.3"]
      }
      masterNodeGroup = {
        replicas = 1
        zones    = ["ru-central1-a"]
        instanceClass = {
          cores   = 2
          memory  = 4096
          imageID = "fd8"
        }
      }
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.layout == "WithoutNAT"
    error_message = "expected layout WithoutNAT"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.existingNetworkID == "network-existing"
    error_message = "expected existingNetworkID to be carried over"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.existingZoneToSubnetIDMap["ru-central1-b"] == "subnet-b"
    error_message = "expected existingZoneToSubnetIDMap to be carried over"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.dhcpOptions.domainName == "example.com"
    error_message = "expected dhcpOptions.domainName to be carried over"
  }

  assert {
    condition     = jsonencode(output.settings.spec.settings.nodes.parameters.dhcpOptions.domainNameServers) == jsonencode(["10.20.0.2", "10.20.0.3"])
    error_message = "expected dhcpOptions.domainNameServers to be carried over"
  }

  assert {
    condition     = jsonencode(output.nodeGroups.master.spec.cloudInstances.zones) == jsonencode(["ru-central1-a"])
    error_message = "expected the master NodeGroup zones to win over the cluster-wide zones"
  }
}

# WithNATInstance carries a whole extra configuration block plus a second
# credential. Losing any of it recreates or breaks the NAT instance.
run "state_a_with_nat_instance" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "WithNATInstance"
      sshPublicKey    = "ssh-rsa NAT_KEY"
      nodeNetworkCIDR = "10.30.0.0/16"
      masterNodeGroup = {
        replicas = 1
        instanceClass = {
          cores   = 2
          memory  = 4096
          imageID = "fd8"
        }
      }
      withNATInstance = {
        exporterAPIKey             = "exporter-key-nat-123"
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

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.layout == "WithNATInstance"
    error_message = "expected layout WithNATInstance"
  }

  assert {
    condition = (
      output.settings.spec.settings.nodes.parameters.withNATInstance.internalSubnetID == "subnet-internal" &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.internalSubnetCIDR == "10.30.4.0/24" &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.externalSubnetID == "subnet-external" &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.natInstanceExternalAddress == "203.0.113.10" &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.natInstanceInternalAddress == "10.30.4.10"
    )
    error_message = "expected the whole withNATInstance block to survive the projection"
  }

  assert {
    condition = (
      output.settings.spec.settings.nodes.parameters.withNATInstance.natInstanceResources.cores == 4 &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.natInstanceResources.memory == 8192 &&
      output.settings.spec.settings.nodes.parameters.withNATInstance.natInstanceResources.platform == "standard-v3"
    )
    error_message = "expected explicit NAT instance sizing to be preserved"
  }

  assert {
    condition = jsonencode(keys(nonsensitive(output.secrets))) == jsonencode([
      "d8-cloud-provider-yandex/d8-credentials",
      "d8-cloud-provider-yandex/d8-credentials-exporter",
    ])
    error_message = "expected both the provider and the exporter credential Secrets"
  }

  assert {
    condition     = nonsensitive(output.secrets)["d8-cloud-provider-yandex/d8-credentials-exporter"].stringData.authScheme == "apiToken"
    error_message = "expected the exporter credential to use the apiToken auth scheme"
  }

  assert {
    condition     = nonsensitive(output.secrets)["d8-cloud-provider-yandex/d8-credentials-exporter"].stringData.secret == "exporter-key-nat-123"
    error_message = "expected the exporter API key to be carried into its Secret"
  }
}

# "Auto" asks the monitoring-service-account module to provision a key; it must
# survive as a literal.
run "state_a_with_nat_instance_auto_exporter_key" {
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
        instanceClass = { cores = 2, memory = 4096, imageID = "fd8" }
      }
      withNATInstance = { exporterAPIKey = "Auto" }
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  assert {
    condition     = nonsensitive(output.secrets)["d8-cloud-provider-yandex/d8-credentials-exporter"].stringData.secret == "Auto"
    error_message = "expected the literal Auto to be preserved"
  }
}

# A PCC without an exporter key must not produce an exporter Secret.
run "state_a_with_nat_instance_without_exporter_key" {
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
        instanceClass = { cores = 2, memory = 4096, imageID = "fd8" }
      }
      withNATInstance = { internalSubnetCIDR = "10.30.4.0/24" }
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  assert {
    condition     = jsonencode(keys(nonsensitive(output.secrets))) == jsonencode(["d8-cloud-provider-yandex/d8-credentials"])
    error_message = "expected no exporter Secret when exporterAPIKey is unset"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.withNATInstance.internalSubnetCIDR == "10.30.4.0/24"
    error_message = "expected a partial withNATInstance block to still be carried over"
  }
}

# Static node groups get their own NodeGroup / YandexInstanceClass pair, with
# per-group zones and external addressing.
run "state_a_multiple_node_groups" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.40.0.0/16"
      zones           = ["ru-central1-a", "ru-central1-b", "ru-central1-d"]
      masterNodeGroup = {
        replicas      = 3
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8master" }
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
            coreFraction        = 20
            externalIPAddresses = ["Auto", "Auto"]
          }
        },
        {
          name         = "system"
          replicas     = 1
          nodeTemplate = { labels = { node-role = "system" } }
          instanceClass = {
            cores   = 2
            memory  = 2048
            imageID = "fd8system"
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

  assert {
    condition     = length(output.nodeGroups) == 3 && length(output.instanceClasses) == 3
    error_message = "expected a NodeGroup and a YandexInstanceClass per PCC node group"
  }

  assert {
    condition     = output.nodeGroups.worker.spec.cloudInstances.classReference.name == "worker-87eba76e7f31"
    error_message = "expected the hashed instance class name for the worker node group"
  }

  assert {
    condition = (
      output.nodeGroups.worker.spec.cloudInstances.minPerZone == 2 &&
      output.nodeGroups.worker.spec.cloudInstances.maxPerZone == 2
    )
    error_message = "expected the worker replicas to be mirrored"
  }

  assert {
    condition     = jsonencode(output.nodeGroups.worker.spec.cloudInstances.zones) == jsonencode(["ru-central1-b"])
    error_message = "expected the worker node group zones to win over the cluster-wide zones"
  }

  assert {
    condition     = jsonencode(output.nodeGroups.system.spec.cloudInstances.zones) == jsonencode(["ru-central1-a", "ru-central1-b", "ru-central1-d"])
    error_message = "expected the system node group to inherit the cluster-wide zones"
  }

  assert {
    condition     = output.nodeGroups.system.spec.nodeTemplate.labels["node-role"] == "system"
    error_message = "expected the PCC nodeTemplate to be carried over"
  }

  assert {
    condition     = output.nodeGroups.worker.spec.nodeTemplate == null
    error_message = "expected no nodeTemplate for a node group that does not define one"
  }

  assert {
    condition     = output.instanceClasses["worker-87eba76e7f31"].spec.coreFraction == 20
    error_message = "expected an explicit coreFraction to be preserved"
  }

  assert {
    condition     = output.instanceClasses["system-bbc5e661e106"].spec.coreFraction == 100
    error_message = "expected an omitted coreFraction to default to 100"
  }

  # etcd disks exist on master nodes only.
  assert {
    condition = (
      output.instanceClasses["worker-87eba76e7f31"].spec.etcdDiskSizeGB == null &&
      output.instanceClasses["system-bbc5e661e106"].spec.etcdDiskSizeGB == null
    )
    error_message = "expected etcdDiskSizeGB to be set on the master instance class only"
  }

  assert {
    condition     = jsonencode(output.settings.spec.settings.nodes.parameters.externalIPAddresses["worker"]) == jsonencode(["Auto", "Auto"])
    error_message = "expected the worker external addresses to be keyed by node group name"
  }

  assert {
    condition     = !contains(keys(output.settings.spec.settings.nodes.parameters.externalIPAddresses), "system")
    error_message = "expected no external addresses entry for a node group without them"
  }
}

# Hybrid clusters keep their static node groups in the PCC but have no
# masterNodeGroup to manage.
run "state_a_hybrid_without_master" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.50.0.0/16"
      masterNodeGroup = {
        replicas      = 0
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8master" }
      }
      nodeGroups = [
        {
          name          = "worker"
          replicas      = 1
          instanceClass = { cores = 2, memory = 2048, imageID = "fd8worker" }
        },
      ]
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  assert {
    condition     = jsonencode(keys(output.nodeGroups)) == jsonencode(["worker"])
    error_message = "expected no master NodeGroup when masterNodeGroup.replicas is zero"
  }

  assert {
    condition     = jsonencode(keys(output.instanceClasses)) == jsonencode(["worker-87eba76e7f31"])
    error_message = "expected no master YandexInstanceClass when masterNodeGroup.replicas is zero"
  }
}

# --- Transition: both sources present ---------------------------------------

# The in-cluster migration created the master pair but not the worker pair yet.
# Switching sources now would plan against an incomplete configuration.
run "transition_partially_migrated_keeps_pcc" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.60.0.0/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8master" }
      }
      nodeGroups = [
        {
          name          = "worker"
          replicas      = 1
          instanceClass = { cores = 2, memory = 2048, imageID = "fd8worker" }
        },
      ]
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
        spec       = { cores = 4, memory = 8192, imageID = "fd8master" }
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
          provider = { parameters = { cloudID = "cloud-1", folderID = "folder-1" } }
          nodes    = { parameters = { sshPublicKey = "ssh-rsa AAAA", layout = "Standard", nodeNetworkCIDR = "10.60.0.0/16" } }
        }
      }
    }
  }

  assert {
    condition     = length(output.nodeGroups) == 2
    error_message = "expected both PCC node groups to be synthesised while migration is incomplete"
  }
}

# All pairs present, ModuleConfig v2 in place: the cluster resources take over
# even though the PCC secret still exists. layout, nodeNetworkCIDR and
# requires that; the provider coordinates and the credentials differ, which is
# what proves the source of truth switched.
run "transition_fully_migrated_switches_to_resources" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa STALE"
      nodeNetworkCIDR = "10.60.0.0/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8master" }
      }
      provider = {
        cloudID            = "cloud-stale"
        folderID           = "folder-stale"
        serviceAccountJSON = "{\"id\":\"sa-stale\"}"
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
        spec       = { cores = 4, memory = 8192, imageID = "fd8master", platformID = "standard-v2", diskType = "network-ssd" }
      }
    }

    secrets = {
      "d8-cloud-provider-yandex/d8-credentials" = {
        apiVersion = "v1"
        kind       = "Secret"
        metadata   = { name = "d8-credentials", namespace = "d8-cloud-provider-yandex" }
        stringData = { authScheme = "serviceAccount", secret = "{\"id\":\"sa-fresh\"}" }
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
          provider = { parameters = { cloudID = "cloud-fresh", folderID = "folder-fresh" } }
          nodes    = { parameters = { sshPublicKey = "ssh-rsa FRESH", layout = "Standard", nodeNetworkCIDR = "10.60.0.0/16" } }
        }
      }
    }
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.cloudID == "cloud-fresh"
    error_message = "expected provider coordinates to come from the ModuleConfig, not from the stale PCC"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.sshPublicKey == "ssh-rsa FRESH"
    error_message = "expected node parameters to come from the ModuleConfig, not from the stale PCC"
  }

  assert {
    condition     = nonsensitive(output.secrets)["d8-cloud-provider-yandex/d8-credentials"].stringData.secret == "{\"id\":\"sa-fresh\"}"
    error_message = "expected credentials to come from the Secret, not from the stale PCC"
  }
}

# The migration is only complete once the ModuleConfig has been converted to
# version 2; a v1 ModuleConfig must not hand over the source of truth.
run "transition_module_config_v1_keeps_pcc" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.60.0.0/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8master" }
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
        spec       = { cores = 4, memory = 8192, imageID = "fd8master" }
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
      spec       = { enabled = true, version = 1 }
    }
  }

  assert {
    condition     = output.settings.spec.version == 2 && output.settings.spec.settings.nodes.parameters.nodeNetworkCIDR == "10.60.0.0/16"
    error_message = "expected a version 1 ModuleConfig to keep the PCC authoritative, so the projection wins"
  }
}

# --- State B: cluster resources ---------------------------------------------

run "state_b_standard" {
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
            zones          = ["ru-central1-a", "ru-central1-b", "ru-central1-d"]
          }
          nodeTemplate = {
            labels = {
              "node-role.kubernetes.io/control-plane" = ""
              "node-role.kubernetes.io/master"        = ""
            }
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
          cores                 = 4
          memory                = 8192
          imageID               = "fd8master"
          platformID            = "standard-v2"
          diskType              = "network-ssd"
          diskSizeGB            = 50
          coreFraction          = 100
          networkType           = "Standard"
          assignPublicIPAddress = false
          etcdDiskSizeGB        = 20
        }
      }
      "worker-87eba76e7f31" = {
        apiVersion = "deckhouse.io/v1"
        kind       = "YandexInstanceClass"
        metadata   = { name = "worker-87eba76e7f31" }
        spec = {
          cores                 = 2
          memory                = 2048
          imageID               = "fd8worker"
          platformID            = "standard-v3"
          diskType              = "network-hdd"
          assignPublicIPAddress = true
          additionalSubnets     = ["subnet-ext"]
          mainSubnet            = "subnet-main"
        }
      }
    }

    secrets = {
      "d8-cloud-provider-yandex/d8-credentials" = {
        apiVersion = "v1"
        kind       = "Secret"
        metadata   = { name = "d8-credentials", namespace = "d8-cloud-provider-yandex" }
        stringData = { authScheme = "serviceAccount", secret = "{\"id\":\"sa-b\"}" }
        type       = "cloud-provider.deckhouse.io/credentials"
      }
    }

    settings = {
      apiVersion = "deckhouse.io/v1alpha1"
      kind       = "ModuleConfig"
      metadata   = { creationTimestamp = null, name = "cloud-provider-yandex" }
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
              zones           = ["ru-central1-a", "ru-central1-b", "ru-central1-d"]
              labels          = { env = "prod" }
              externalIPAddresses = {
                worker = ["Auto"]
              }
            }
          }
          storage = { parameters = { excludedStorageClasses = ["network-hdd"] } }
          ccm     = { parameters = { additionalExternalNetworkIDs = ["network-ext"] } }
        }
      }
    }
  }

  assert {
    condition     = jsonencode(output.settings.spec.settings.storage.parameters.excludedStorageClasses) == jsonencode(["network-hdd"])
    error_message = "expected the ModuleConfig to pass through unchanged"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.nodeNetworkCIDR == "10.70.0.0/16" && output.settings.spec.settings.nodes.parameters.layout == "Standard"
    error_message = "expected node parameters to be read from the ModuleConfig"
  }

  assert {
    condition     = jsonencode(output.settings.spec.settings.nodes.parameters.labels) == jsonencode({ env = "prod" })
    error_message = "expected cluster-wide labels from the ModuleConfig"
  }

  assert {
    condition     = jsonencode(output.settings.spec.settings.nodes.parameters.externalIPAddresses["worker"]) == jsonencode(["Auto"])
    error_message = "expected per-node-group external addresses from the ModuleConfig"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.etcdDiskSizeGB == 20
    error_message = "expected the cluster instance class to pass through unchanged"
  }

  assert {
    condition     = output.nodeGroups.worker.spec.cloudInstances.classReference.name == "worker-87eba76e7f31"
    error_message = "expected the cluster NodeGroup to pass through unchanged"
  }

  assert {
    condition     = nonsensitive(output.secrets)["d8-cloud-provider-yandex/d8-credentials"].stringData.secret == "{\"id\":\"sa-b\"}"
    error_message = "expected the credential Secret to pass through unchanged"
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.cloudID == "cloud-b" && output.settings.spec.settings.provider.parameters.folderID == "folder-b"
    error_message = "expected provider coordinates from the ModuleConfig"
  }
}

# dhctl base64-encodes Secret values that are not valid UTF-8 into `data`;
# credentials must be resolvable from either field.
run "state_b_credentials_from_base64_data" {
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
        spec       = { cores = 2, memory = 4096, imageID = "fd8" }
      }
    }

    secrets = {
      "d8-cloud-provider-yandex/d8-credentials" = {
        apiVersion = "v1"
        kind       = "Secret"
        metadata   = { name = "d8-credentials", namespace = "d8-cloud-provider-yandex" }
        data       = { authScheme = "c2VydmljZUFjY291bnQ=", secret = "eyJpZCI6InNhLWI2NCJ9" } // gitleaks:allow
        type       = "cloud-provider.deckhouse.io/credentials"
      }
      "d8-cloud-provider-yandex/d8-credentials-exporter" = {
        apiVersion = "v1"
        kind       = "Secret"
        metadata   = { name = "d8-credentials-exporter", namespace = "d8-cloud-provider-yandex" }
        data       = { authScheme = "YXBpVG9rZW4=", secret = "ZXhwb3J0ZXItYjY0" } // gitleaks:allow
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
          nodes    = { parameters = { sshPublicKey = "ssh-rsa AAAA", layout = "WithNATInstance", nodeNetworkCIDR = "10.80.0.0/16" } }
        }
      }
    }
  }

  assert {
    condition     = base64decode(nonsensitive(output.secrets)["d8-cloud-provider-yandex/d8-credentials"].data.secret) == "{\"id\":\"sa-b64\"}"
    error_message = "expected a Secret carrying base64 data to pass through unchanged"
  }

  assert {
    condition     = base64decode(nonsensitive(output.secrets)["d8-cloud-provider-yandex/d8-credentials-exporter"].data.secret) == "exporter-b64"
    error_message = "expected the exporter Secret carrying base64 data to pass through unchanged"
  }
}

# Some code paths key the Secret map by bare name; both shapes must resolve.
run "state_b_credentials_from_name_keyed_secrets" {
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
        spec       = { cores = 2, memory = 4096, imageID = "fd8" }
      }
    }

    secrets = {
      d8-credentials = {
        apiVersion = "v1"
        kind       = "Secret"
        stringData = { authScheme = "serviceAccount", secret = "{\"id\":\"sa-name-keyed\"}" }
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
          nodes    = { parameters = { sshPublicKey = "ssh-rsa AAAA", layout = "WithoutNAT", nodeNetworkCIDR = "10.90.0.0/16" } }
        }
      }
    }
  }

  assert {
    condition     = nonsensitive(output.secrets)["d8-credentials"].stringData.secret == "{\"id\":\"sa-name-keyed\"}"
    error_message = "expected a Secret keyed by bare name to pass through too"
  }
}

# --- Destroy / unconfigured -------------------------------------------------

# Nothing is configured: the module must resolve to empty values rather than
# fail, so destroy runs and early bootstrap steps are not blocked.
run "destroy_no_inputs" {
  command = plan

  assert {
    condition     = length(output.nodeGroups) == 0 && length(output.instanceClasses) == 0
    error_message = "expected no resources to be resolved"
  }

  assert {
    condition     = length(nonsensitive(output.secrets)) == 0
    error_message = "expected no credential Secrets to be resolved"
  }

  assert {
    condition     = output.settings == null
    error_message = "expected no ModuleConfig to be resolved"
  }

  assert {
    condition     = try(output.settings.spec.settings.nodes.parameters, null) == null
    error_message = "expected no node parameters to be resolved"
  }

  assert {
    condition     = try(output.settings.spec.settings.provider.parameters, null) == null
    error_message = "expected no provider coordinates to be resolved"
  }
}

# --- Negative scenarios -----------------------------------------------------

# Restores the nodeNetworkCIDR validation that used to live in every layout's
# variables.tf.
run "negative_node_network_cidr_is_not_a_network_address" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.0.0.1/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 2, memory = 4096, imageID = "fd8" }
      }
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  expect_failures = [output.settings]
}

run "negative_node_network_cidr_is_malformed" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "not-a-cidr"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 2, memory = 4096, imageID = "fd8" }
      }
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  expect_failures = [output.settings]
}

run "negative_node_network_cidr_missing" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion   = "deckhouse.io/v1"
      kind         = "YandexClusterConfiguration"
      layout       = "Standard"
      sshPublicKey = "ssh-rsa AAAA"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 2, memory = 4096, imageID = "fd8" }
      }
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  expect_failures = [output.settings]
}

run "negative_unknown_layout" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "SomethingElse"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.0.0.0/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 2, memory = 4096, imageID = "fd8" }
      }
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  expect_failures = [output.settings]
}

# A ModuleConfig that was never converted to version 2 and no PCC: there is no
# usable configuration at all.
run "negative_module_config_v1_without_pcc" {
  command = plan

  variables {
    settings = {
      apiVersion = "deckhouse.io/v1alpha1"
      kind       = "ModuleConfig"
      metadata   = { name = "cloud-provider-yandex" }
      spec       = { enabled = true, version = 1 }
    }
  }

  expect_failures = [output.settings]
}

run "negative_missing_service_account_key" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.0.0.0/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 2, memory = 4096, imageID = "fd8" }
      }
      provider = {
        cloudID  = "cloud-1"
        folderID = "folder-1"
      }
    }
  }

  expect_failures = [output.settings]
}

# A NodeGroup whose YandexInstanceClass was deleted would otherwise plan a
# machine with zero cores.
run "negative_dangling_class_reference" {
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
            classReference = { kind = "YandexInstanceClass", name = "master-deleted" }
            minPerZone     = 1
            maxPerZone     = 1
          }
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
          provider = { parameters = { cloudID = "cloud-1", folderID = "folder-1" } }
          nodes    = { parameters = { sshPublicKey = "ssh-rsa AAAA", layout = "Standard", nodeNetworkCIDR = "10.0.0.0/16" } }
        }
      }
    }
  }

  expect_failures = [output.settings]
}

run "negative_foreign_class_reference_kind" {
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
            classReference = { kind = "AWSInstanceClass", name = "master-fc613b4dfd67" }
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
        spec       = { cores = 2, memory = 4096, imageID = "fd8" }
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
          provider = { parameters = { cloudID = "cloud-1", folderID = "folder-1" } }
          nodes    = { parameters = { sshPublicKey = "ssh-rsa AAAA", layout = "Standard", nodeNetworkCIDR = "10.0.0.0/16" } }
        }
      }
    }
  }

  expect_failures = [output.settings]
}

# buildMigrationResources() rejects these too; failing here keeps terraform and
# the in-cluster migration consistent.
run "negative_node_group_with_empty_name" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.0.0.0/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 2, memory = 4096, imageID = "fd8" }
      }
      nodeGroups = [
        {
          name          = ""
          replicas      = 1
          instanceClass = { cores = 2, memory = 2048, imageID = "fd8worker" }
        },
      ]
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  expect_failures = [output.settings]
}

run "negative_duplicate_node_group_names" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.0.0.0/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 2, memory = 4096, imageID = "fd8" }
      }
      nodeGroups = [
        {
          name          = "worker"
          replicas      = 1
          instanceClass = { cores = 2, memory = 2048, imageID = "fd8worker" }
        },
        {
          name          = "worker"
          replicas      = 2
          instanceClass = { cores = 4, memory = 4096, imageID = "fd8worker" }
        },
      ]
      provider = {
        cloudID            = "cloud-1"
        folderID           = "folder-1"
        serviceAccountJSON = "{\"id\":\"sa\"}"
      }
    }
  }

  expect_failures = [output.settings]
}

# The exporter Secret carries the same credentials type as the primary one. Both Go surfaces look
# the primary Secret up by name, so an exporter-only cluster is not migrated and must keep the PCC.
run "transition_exporter_credentials_only_keeps_pcc" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa STALE"
      nodeNetworkCIDR = "10.60.0.0/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8master" }
      }
      provider = {
        cloudID            = "cloud-stale"
        folderID           = "folder-stale"
        serviceAccountJSON = "{\"id\":\"sa-stale\"}"
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
        spec       = { cores = 4, memory = 8192, imageID = "fd8master", platformID = "standard-v2", diskType = "network-ssd" }
      }
    }

    secrets = {
      "d8-cloud-provider-yandex/d8-credentials-exporter" = {
        apiVersion = "v1"
        kind       = "Secret"
        metadata   = { name = "d8-credentials-exporter", namespace = "d8-cloud-provider-yandex" }
        stringData = { authScheme = "apiToken", secret = "exporter-key" }
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
          provider = { parameters = { cloudID = "cloud-fresh", folderID = "folder-fresh" } }
          nodes    = { parameters = { sshPublicKey = "ssh-rsa FRESH", layout = "Standard", nodeNetworkCIDR = "10.60.0.0/16" } }
        }
      }
    }
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.cloudID == "cloud-stale"
    error_message = "expected the PCC to stay the source of truth while the primary credential Secret is missing"
  }
}

# A PCC that declares no node groups has nothing left to apply: the hook agrees, because its loop
# over the PCC node groups does not run, so the new resources have to take over.
run "transition_pcc_without_node_groups_switches_to_resources" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa STALE"
      nodeNetworkCIDR = "10.60.0.0/16"
      provider = {
        cloudID            = "cloud-stale"
        folderID           = "folder-stale"
        serviceAccountJSON = "{\"id\":\"sa-stale\"}"
      }
    }

    secrets = {
      "d8-cloud-provider-yandex/d8-credentials" = {
        apiVersion = "v1"
        kind       = "Secret"
        metadata   = { name = "d8-credentials", namespace = "d8-cloud-provider-yandex" }
        stringData = { authScheme = "serviceAccount", secret = "{\"id\":\"sa-fresh\"}" }
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
          provider = { parameters = { cloudID = "cloud-fresh", folderID = "folder-fresh" } }
          nodes    = { parameters = { sshPublicKey = "ssh-rsa FRESH", layout = "Standard", nodeNetworkCIDR = "10.60.0.0/16" } }
        }
      }
    }
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.cloudID == "cloud-fresh"
    error_message = "expected the new resources to take over when the PCC declares no node groups"
  }
}

# A ModuleConfig with `spec.enabled: false` is not a source of truth for
# IsMigrationResourcesApplied() in hooks/internal/migration.go, so terraform must
# not treat it as one either: everything else about this cluster looks migrated.
run "transition_module_config_disabled_keeps_pcc" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.60.0.0/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8master" }
      }
      provider = {
        cloudID            = "cloud-pcc"
        folderID           = "folder-pcc"
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
        spec       = { cores = 4, memory = 8192, imageID = "fd8master" }
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
        enabled = false
        version = 2
        settings = {
          provider = { parameters = { cloudID = "cloud-mc", folderID = "folder-mc" } }
          nodes    = { parameters = { sshPublicKey = "ssh-rsa AAAA", layout = "Standard", nodeNetworkCIDR = "10.60.0.0/16" } }
        }
      }
    }
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.cloudID == "cloud-pcc"
    error_message = "expected a disabled ModuleConfig to keep the PCC authoritative"
  }
}

# A ModuleConfig v2 with no settings at all, which the hook rejects
# (len(SettingsV2) == 0). Accepting it here would resolve the layout, the
# nodeNetworkCIDR and every node group to nothing.
run "transition_module_config_without_settings_keeps_pcc" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion      = "deckhouse.io/v1"
      kind            = "YandexClusterConfiguration"
      layout          = "Standard"
      sshPublicKey    = "ssh-rsa AAAA"
      nodeNetworkCIDR = "10.60.0.0/16"
      masterNodeGroup = {
        replicas      = 1
        instanceClass = { cores = 4, memory = 8192, imageID = "fd8master" }
      }
      provider = {
        cloudID            = "cloud-pcc"
        folderID           = "folder-pcc"
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
        spec       = { cores = 4, memory = 8192, imageID = "fd8master" }
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
      spec       = { enabled = true, version = 2 }
    }
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.cloudID == "cloud-pcc"
    error_message = "expected a ModuleConfig without settings to keep the PCC authoritative"
  }
}

# dhctl omits providerClusterConfiguration on a ModuleConfig-only cluster, but an
# empty object must be treated the same way: taking the PCC branch here would
# resolve every value to empty and fail validation on a healthy cluster.
run "state_b_empty_pcc_object_is_no_pcc" {
  command = plan

  variables {
    providerClusterConfiguration = {}

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
        spec       = { cores = 4, memory = 8192, imageID = "fd8master" }
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
          provider = { parameters = { cloudID = "cloud-mc", folderID = "folder-mc" } }
          nodes    = { parameters = { sshPublicKey = "ssh-rsa AAAA", layout = "Standard", nodeNetworkCIDR = "10.70.0.0/16" } }
        }
      }
    }
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.cloudID == "cloud-mc"
    error_message = "expected an empty providerClusterConfiguration to be treated as absent"
  }

  assert {
    condition     = keys(output.nodeGroups) == ["master"] && keys(output.instanceClasses) == ["master-fc613b4dfd67"]
    error_message = "expected the cluster resources to pass through unchanged"
  }
}
