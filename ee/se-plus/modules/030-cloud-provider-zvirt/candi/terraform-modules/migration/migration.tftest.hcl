# Copyright 2026 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variables {
  providerClusterConfiguration = {
    apiVersion   = "deckhouse.io/v1"
    kind         = "ZvirtClusterConfiguration"
    layout       = "Standard"
    sshPublicKey = "ssh-rsa AAAA"
    clusterID    = "b46372e7-0d52-40c7-9bbf-fda31e187088"
    masterNodeGroup = {
      replicas = 3
      instanceClass = {
        numCPUs         = 4
        memory          = 8192
        template        = "debian-bookworm"
        vnicProfileID   = "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
        storageDomainID = "c4bf82a5-b803-40c3-9f6c-b9398378f424"
      }
    }
    provider = {
      server   = "https://zvirt.example.com/ovirt-engine/api"
      username = "admin@internal"
      password = "s3cret"
      insecure = true
    }
  }
}

run "pcc_is_the_source_of_truth_when_nothing_is_migrated" {
  command = plan

  assert {
    condition     = output.settings.spec.version == 2
    error_message = "the projected ModuleConfig must be version 2"
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.server == "https://zvirt.example.com/ovirt-engine/api"
    error_message = "provider.parameters.server must come from the legacy configuration"
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.clusterID == "b46372e7-0d52-40c7-9bbf-fda31e187088"
    error_message = "the top-level clusterID must land in provider.parameters"
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.insecure == true
    error_message = "provider.parameters.insecure must be carried over"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.sshPublicKey == "ssh-rsa AAAA"
    error_message = "nodes.parameters.sshPublicKey must come from the legacy configuration"
  }

  assert {
    condition     = output.settings.spec.settings.nodes.parameters.layout == "Standard"
    error_message = "nodes.parameters.layout must come from the legacy configuration"
  }

  # The credentials leave the ModuleConfig and land in the managed Secret instead.
  assert {
    condition     = !can(output.settings.spec.settings.provider.parameters.username)
    error_message = "the login must not be part of the ModuleConfig"
  }
}

run "master_node_group_and_instance_class_are_projected" {
  command = plan

  assert {
    condition     = length(keys(output.nodeGroups)) == 1
    error_message = "a legacy configuration with only a masterNodeGroup must yield exactly one NodeGroup"
  }

  assert {
    condition     = output.nodeGroups["master"].spec.nodeType == "CloudPermanent"
    error_message = "a migrated master NodeGroup must be CloudPermanent"
  }

  assert {
    condition = (
      output.nodeGroups["master"].spec.cloudInstances.minPerZone == 3 &&
      output.nodeGroups["master"].spec.cloudInstances.maxPerZone == 3
    )
    error_message = "replicas must become min/maxPerZone"
  }

  assert {
    condition     = output.nodeGroups["master"].spec.cloudInstances.classReference.kind == "ZvirtInstanceClass"
    error_message = "the NodeGroup must reference a ZvirtInstanceClass"
  }

  # Mirrors go_lib/cloud-provider/api.BuildInstanceClassName("master").
  assert {
    condition     = output.nodeGroups["master"].spec.cloudInstances.classReference.name == "master-fc613b4dfd67"
    error_message = "the instance class name must match BuildInstanceClassName, got ${output.nodeGroups["master"].spec.cloudInstances.classReference.name}"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.template == "debian-bookworm"
    error_message = "the instance class must carry the template of the legacy configuration"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.storageDomainID == "c4bf82a5-b803-40c3-9f6c-b9398378f424"
    error_message = "storageDomainID must move into the instance class"
  }
}

# The disk sizes must be the ones the previous terraform code defaulted to, not the ones the CRD
# documents: a cluster that never set them must keep its disks.
run "disk_sizes_fall_back_to_the_terraform_defaults" {
  command = plan

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.rootDiskSizeGb == 50
    error_message = "rootDiskSizeGb must default to the pre-migration terraform value of 50"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.etcdDiskSizeGb == 10
    error_message = "etcdDiskSizeGb must default to the pre-migration terraform value of 10"
  }
}

run "explicit_disk_sizes_win_over_the_defaults" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion   = "deckhouse.io/v1"
      kind         = "ZvirtClusterConfiguration"
      layout       = "Standard"
      sshPublicKey = "ssh-rsa AAAA"
      clusterID    = "b46372e7-0d52-40c7-9bbf-fda31e187088"
      masterNodeGroup = {
        replicas = 1
        instanceClass = {
          numCPUs         = 4
          memory          = 8192
          template        = "debian-bookworm"
          vnicProfileID   = "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
          storageDomainID = "c4bf82a5-b803-40c3-9f6c-b9398378f424"
          rootDiskSizeGb  = 120
          etcdDiskSizeGb  = 25
        }
      }
      provider = {
        server   = "https://zvirt.example.com/ovirt-engine/api"
        username = "admin@internal"
        password = "s3cret"
        insecure = true
      }
    }
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.rootDiskSizeGb == 120
    error_message = "an explicit rootDiskSizeGb must be preserved"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.etcdDiskSizeGb == 25
    error_message = "an explicit etcdDiskSizeGb must be preserved"
  }
}

run "worker_node_groups_get_no_etcd_disk" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion   = "deckhouse.io/v1"
      kind         = "ZvirtClusterConfiguration"
      layout       = "Standard"
      sshPublicKey = "ssh-rsa AAAA"
      clusterID    = "b46372e7-0d52-40c7-9bbf-fda31e187088"
      masterNodeGroup = {
        replicas = 3
        instanceClass = {
          numCPUs         = 4
          memory          = 8192
          template        = "debian-bookworm"
          vnicProfileID   = "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
          storageDomainID = "c4bf82a5-b803-40c3-9f6c-b9398378f424"
        }
      }
      nodeGroups = [
        {
          name     = "worker"
          replicas = 2
          instanceClass = {
            numCPUs       = 4
            memory        = 8192
            template      = "debian-bookworm"
            vnicProfileID = "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
          }
        }
      ]
      provider = {
        server   = "https://zvirt.example.com/ovirt-engine/api"
        username = "admin@internal"
        password = "s3cret"
        insecure = true
      }
    }
  }

  assert {
    condition     = length(keys(output.nodeGroups)) == 2
    error_message = "both the master and the worker node group must be projected"
  }

  # A dedicated etcd disk belongs to the master alone.
  assert {
    condition     = output.instanceClasses[output.nodeGroups["worker"].spec.cloudInstances.classReference.name].spec.etcdDiskSizeGb == null
    error_message = "a worker instance class must not carry an etcd disk"
  }
}

# The legacy configuration stores DNS servers as one space-separated string, while the
# InstanceClass takes a list — the same split splitDNSServers() does in the hook.
run "custom_network_config_is_projected_and_dns_servers_are_split" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion   = "deckhouse.io/v1"
      kind         = "ZvirtClusterConfiguration"
      layout       = "Standard"
      sshPublicKey = "ssh-rsa AAAA"
      clusterID    = "b46372e7-0d52-40c7-9bbf-fda31e187088"
      masterNodeGroup = {
        replicas = 1
        instanceClass = {
          numCPUs         = 4
          memory          = 8192
          template        = "debian-bookworm"
          vnicProfileID   = "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
          storageDomainID = "c4bf82a5-b803-40c3-9f6c-b9398378f424"
          customNetworkConfig = {
            networkInterfaceName    = "enp1s0"
            networkInterfaceAddress = ["192.168.1.10", "192.168.1.11"]
            networkInterfaceNetmask = "255.255.255.0"
            networkInterfaceGateway = "192.168.1.1"
            dnsServers              = "8.8.8.8 8.8.4.4"
          }
        }
      }
      provider = {
        server   = "https://zvirt.example.com/ovirt-engine/api"
        username = "admin@internal"
        password = "s3cret"
        insecure = true
      }
    }
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.customNetworkConfig.networkInterfaceName == "enp1s0"
    error_message = "customNetworkConfig must be carried over"
  }

  assert {
    condition     = jsonencode(output.instanceClasses["master-fc613b4dfd67"].spec.customNetworkConfig.dnsServers) == jsonencode(["8.8.8.8", "8.8.4.4"])
    error_message = "dnsServers must be split into a list"
  }
}

run "credentials_are_projected_into_a_managed_secret" {
  command = plan

  assert {
    condition     = output.secrets["d8-cloud-provider-zvirt/d8-credentials"].type == "cloud-provider.deckhouse.io/credentials"
    error_message = "the projected Secret must carry the managed credentials type"
  }

  assert {
    condition     = output.secrets["d8-cloud-provider-zvirt/d8-credentials"].stringData.authScheme == "userPassword"
    error_message = "zVirt authenticates with a login and a password"
  }

  assert {
    condition = (
      output.credentials["d8-credentials"].identity == "admin@internal" &&
      output.credentials["d8-credentials"].secret == "s3cret"
    )
    error_message = "the resolved credentials must come from the legacy configuration"
  }
}

# State B: the bundle has been applied, so the cluster resources win and the legacy configuration
# is ignored even though it is still present.
run "cluster_resources_win_once_the_migration_is_complete" {
  command = plan

  variables {
    settings = {
      apiVersion = "deckhouse.io/v1alpha1"
      kind       = "ModuleConfig"
      metadata   = { name = "cloud-provider-zvirt" }
      spec = {
        enabled = true
        version = 2
        settings = {
          provider = {
            parameters = {
              server    = "https://migrated.example.com/ovirt-engine/api"
              clusterID = "11111111-2222-3333-4444-555555555555"
            }
          }
          nodes = {
            parameters = {
              sshPublicKey = "ssh-rsa MIGRATED"
              layout       = "Standard"
            }
          }
        }
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
            classReference = { kind = "ZvirtInstanceClass", name = "master-fc613b4dfd67" }
            minPerZone     = 3
            maxPerZone     = 3
          }
        }
      }
    }

    instanceClasses = {
      "master-fc613b4dfd67" = {
        apiVersion = "deckhouse.io/v1"
        kind       = "ZvirtInstanceClass"
        metadata   = { name = "master-fc613b4dfd67" }
        spec = {
          numCPUs         = 8
          memory          = 16384
          template        = "migrated-template"
          vnicProfileID   = "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
          storageDomainID = "c4bf82a5-b803-40c3-9f6c-b9398378f424"
          rootDiskSizeGb  = 80
          etcdDiskSizeGb  = 20
        }
      }
    }

    secrets = {
      "d8-cloud-provider-zvirt/d8-credentials" = {
        apiVersion = "v1"
        kind       = "Secret"
        metadata   = { name = "d8-credentials", namespace = "d8-cloud-provider-zvirt" }
        type       = "cloud-provider.deckhouse.io/credentials"
        stringData = {
          authScheme = "userPassword"
          identity   = "migrated@internal"
          secret     = "migrated-password"
        }
      }
    }
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.server == "https://migrated.example.com/ovirt-engine/api"
    error_message = "a completed migration must read the cluster ModuleConfig, not the legacy configuration"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.template == "migrated-template"
    error_message = "a completed migration must read the cluster instance class"
  }

  assert {
    condition     = output.credentials["d8-credentials"].identity == "migrated@internal"
    error_message = "a completed migration must read the cluster credential Secret"
  }
}

# A half-applied migration must keep reading the legacy configuration, otherwise terraform would
# flap between two incomplete sources.
run "an_incomplete_migration_keeps_reading_the_legacy_configuration" {
  command = plan

  variables {
    settings = {
      apiVersion = "deckhouse.io/v1alpha1"
      kind       = "ModuleConfig"
      metadata   = { name = "cloud-provider-zvirt" }
      spec = {
        enabled = true
        version = 2
        settings = {
          provider = { parameters = { server = "https://migrated.example.com/ovirt-engine/api", clusterID = "11111111-2222-3333-4444-555555555555" } }
          nodes    = { parameters = { sshPublicKey = "ssh-rsa MIGRATED", layout = "Standard" } }
        }
      }
    }
    # The NodeGroup exists, but its instance class does not, and there is no credential Secret.
    nodeGroups = {
      master = {
        apiVersion = "deckhouse.io/v1"
        kind       = "NodeGroup"
        metadata   = { name = "master" }
        spec = {
          nodeType       = "CloudPermanent"
          cloudInstances = { classReference = { kind = "ZvirtInstanceClass", name = "master-fc613b4dfd67" } }
        }
      }
    }
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.server == "https://zvirt.example.com/ovirt-engine/api"
    error_message = "an incomplete migration must keep the legacy configuration as the source of truth"
  }
}

# A ModuleConfig-only cluster: no legacy configuration at all.
run "module_config_only_cluster_needs_no_legacy_configuration" {
  command = plan

  variables {
    providerClusterConfiguration = null

    settings = {
      apiVersion = "deckhouse.io/v1alpha1"
      kind       = "ModuleConfig"
      metadata   = { name = "cloud-provider-zvirt" }
      spec = {
        enabled = true
        version = 2
        settings = {
          provider = { parameters = { server = "https://new.example.com/ovirt-engine/api", clusterID = "11111111-2222-3333-4444-555555555555" } }
          nodes    = { parameters = { sshPublicKey = "ssh-rsa NEW", layout = "Standard" } }
        }
      }
    }

    secrets = {
      "d8-cloud-provider-zvirt/d8-credentials" = {
        apiVersion = "v1"
        kind       = "Secret"
        metadata   = { name = "d8-credentials", namespace = "d8-cloud-provider-zvirt" }
        type       = "cloud-provider.deckhouse.io/credentials"
        stringData = { authScheme = "userPassword", identity = "new@internal", secret = "new-password" }
      }
    }
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.server == "https://new.example.com/ovirt-engine/api"
    error_message = "a ModuleConfig-only cluster must read its ModuleConfig"
  }

  assert {
    condition     = length(keys(output.nodeGroups)) == 0
    error_message = "a ModuleConfig-only cluster without NodeGroups must project none"
  }
}

# A hybrid cluster: the legacy configuration carries no masterNodeGroup, so no master NodeGroup is
# projected for it.
run "hybrid_cluster_gets_no_master_node_group" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion   = "deckhouse.io/v1"
      kind         = "ZvirtClusterConfiguration"
      layout       = "Standard"
      sshPublicKey = "ssh-rsa AAAA"
      clusterID    = "b46372e7-0d52-40c7-9bbf-fda31e187088"
      provider = {
        server   = "https://zvirt.example.com/ovirt-engine/api"
        username = "admin@internal"
        password = "s3cret"
      }
    }
  }

  assert {
    condition     = length(keys(output.nodeGroups)) == 0
    error_message = "a legacy configuration without a masterNodeGroup must project no NodeGroup"
  }

  assert {
    condition     = output.settings.spec.settings.provider.parameters.server == "https://zvirt.example.com/ovirt-engine/api"
    error_message = "the provider settings must still be projected for a hybrid cluster"
  }
}

# Nothing configured at all: a destroy run must not fail the preconditions.
run "an_unconfigured_run_is_allowed" {
  command = plan

  variables {
    providerClusterConfiguration = null
    settings                     = null
  }

  assert {
    condition     = length(keys(output.nodeGroups)) == 0
    error_message = "an unconfigured run must resolve to no node groups"
  }
}

# Backward compatibility with the pre-migration terraform code.
#
# Before the migration the node modules read the legacy configuration directly. They now read the
# resolved InstanceClass instead, so the values they derive from it have to stay bit-for-bit the
# same — a changed disk size or VM name recreates the node. The numbers below were captured from
# the pre-migration code by running its locals against this very configuration.
run "consumer_values_match_the_pre_migration_terraform" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion   = "deckhouse.io/v1"
      kind         = "ZvirtClusterConfiguration"
      layout       = "Standard"
      sshPublicKey = "ssh-rsa AAAA"
      clusterID    = "b46372e7-0d52-40c7-9bbf-fda31e187088"
      masterNodeGroup = {
        replicas = 3
        instanceClass = {
          numCPUs         = 4
          memory          = 8192
          template        = "debian-bookworm"
          vnicProfileID   = "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
          storageDomainID = "c4bf82a5-b803-40c3-9f6c-b9398378f424"
          customNetworkConfig = {
            networkInterfaceName    = "enp1s0"
            networkInterfaceAddress = ["192.168.1.10", "192.168.1.11"]
            networkInterfaceNetmask = "255.255.255.0"
            networkInterfaceGateway = "192.168.1.1"
            dnsServers              = "8.8.8.8 8.8.4.4"
          }
        }
      }
      nodeGroups = [
        {
          name     = "worker"
          replicas = 2
          instanceClass = {
            numCPUs        = 2
            memory         = 4096
            template       = "debian-bookworm"
            vnicProfileID  = "49bb4594-0cd4-4eb7-8288-8594eafd5a86"
            rootDiskSizeGb = 80
          }
        }
      ]
      provider = {
        server   = "https://zvirt.example.com/ovirt-engine/api"
        username = "admin@internal"
        password = "s3cret"
        insecure = true
      }
    }
  }

  # master-node: every value its locals derive from the instance class.
  assert {
    condition = (
      output.instanceClasses["master-fc613b4dfd67"].spec.vnicProfileID == "49bb4594-0cd4-4eb7-8288-8594eafd5a86" &&
      output.instanceClasses["master-fc613b4dfd67"].spec.storageDomainID == "c4bf82a5-b803-40c3-9f6c-b9398378f424" &&
      output.instanceClasses["master-fc613b4dfd67"].spec.template == "debian-bookworm" &&
      output.instanceClasses["master-fc613b4dfd67"].spec.numCPUs == 4 &&
      output.instanceClasses["master-fc613b4dfd67"].spec.memory == 8192
    )
    error_message = "the master instance class must keep the values the pre-migration code read from the legacy configuration"
  }

  # The node modules multiply these by 1024^3; the products are the sizes the pre-migration code
  # passed to ovirt_disk_resize and ovirt_disk.
  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.rootDiskSizeGb * 1024 * 1024 * 1024 == 53687091200
    error_message = "the master boot disk must keep its pre-migration size of 53687091200 bytes"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.etcdDiskSizeGb * 1024 * 1024 * 1024 == 10737418240
    error_message = "the etcd disk must keep its pre-migration size of 10737418240 bytes"
  }

  # The pre-migration code handed initialization_dns the raw string, so joining the list back has
  # to reproduce it exactly.
  assert {
    condition     = join(" ", output.instanceClasses["master-fc613b4dfd67"].spec.customNetworkConfig.dnsServers) == "8.8.8.8 8.8.4.4"
    error_message = "joining dnsServers back must reproduce the string the pre-migration code used"
  }

  # Addresses are indexed by node index, so their order has to survive the projection.
  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].spec.customNetworkConfig.networkInterfaceAddress[1] == "192.168.1.11"
    error_message = "networkInterfaceAddress is indexed by node index and must keep its order"
  }

  # static-node: an explicit rootDiskSizeGb and a node group without customNetworkConfig.
  assert {
    condition = (
      output.instanceClasses["worker-87eba76e7f31"].spec.numCPUs == 2 &&
      output.instanceClasses["worker-87eba76e7f31"].spec.memory == 4096 &&
      output.instanceClasses["worker-87eba76e7f31"].spec.rootDiskSizeGb * 1024 * 1024 * 1024 == 85899345920
    )
    error_message = "the worker instance class must keep the values the pre-migration code read"
  }

  assert {
    condition     = output.instanceClasses["worker-87eba76e7f31"].spec.customNetworkConfig == null
    error_message = "a node group without customNetworkConfig must not gain one"
  }

  # clusterID and sshPublicKey were read straight off the legacy configuration.
  assert {
    condition = (
      output.settings.spec.settings.provider.parameters.clusterID == "b46372e7-0d52-40c7-9bbf-fda31e187088" &&
      output.settings.spec.settings.nodes.parameters.sshPublicKey == "ssh-rsa AAAA"
    )
    error_message = "clusterID and sshPublicKey must keep their pre-migration values"
  }
}
