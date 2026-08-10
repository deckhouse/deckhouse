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

# Scenario 1: post-migration state — PCC absent, real resources present, MC version=1.
# use_pcc = false (has_pcc=false), passthrough mode.
run "no_pcc_with_resources" {
  command = plan

  variables {
    providerClusterConfiguration = null

    nodeGroups = {
      master = {
        apiVersion = "deckhouse.io/v1"
        kind       = "NodeGroup"
        metadata   = { name = "master" }
        spec = {
          cloudInstances = {
            classReference = {
              kind = "DVPInstanceClass"
              name = "master-dvp"
            }
            maxPerZone = 1
            minPerZone = 1
            zones      = ["default"]
          }
          nodeTemplate = {
            labels = {
              "node-role.kubernetes.io/control-plane" = ""
              "node-role.kubernetes.io/master"        = ""
            }
          }
          nodeType = "CloudPermanent"
        }
      }
      worker = {
        apiVersion = "deckhouse.io/v1"
        kind       = "NodeGroup"
        metadata   = { name = "worker" }
        spec = {
          cloudInstances = {
            classReference = {
              kind = "DVPInstanceClass"
              name = "worker-dvp"
            }
            maxPerZone = 0
            minPerZone = 0
            zones      = ["default"]
          }
          nodeType = "CloudPermanent"
        }
      }
    }

    instanceClasses = {
      master-dvp = {
        apiVersion = "deckhouse.io/v1alpha1"
        kind       = "DVPInstanceClass"
        metadata   = { name = "master-dvp" }
        spec = {
          etcdDisk = { size = "5Gi", storageClass = "replicated" }
          rootDisk = {
            image        = { kind = "ClusterVirtualImage", name = "ubuntu-24-04-lts" }
            size         = "50Gi"
            storageClass = "replicated"
          }
          virtualMachine = {
            bootloader              = "EFI"
            cpu                     = { coreFraction = "20%", cores = 4 }
            liveMigrationPolicy     = "PreferForced"
            memory                  = { size = "8Gi" }
            runPolicy               = "AlwaysOnUnlessStoppedManually"
            virtualMachineClassName = "amd-epyc-gen-3"
          }
        }
      }
      worker-dvp = {
        apiVersion = "deckhouse.io/v1alpha1"
        kind       = "DVPInstanceClass"
        metadata   = { name = "worker-dvp" }
        spec = {
          rootDisk = {
            image = { kind = "ClusterVirtualImage", name = "ubuntu-24-04-lts" }
            size  = "40Gi"
          }
          virtualMachine = {
            bootloader              = "EFI"
            cpu                     = { coreFraction = "20%", cores = 4 }
            liveMigrationPolicy     = "PreferForced"
            memory                  = { size = "4Gi" }
            runPolicy               = "AlwaysOnUnlessStoppedManually"
            virtualMachineClassName = "amd-epyc-gen-3"
          }
        }
      }
      ubuntu = {
        apiVersion = "deckhouse.io/v1alpha1"
        kind       = "DVPInstanceClass"
        metadata   = { name = "ubuntu" }
        spec = {
          rootDisk = {
            image        = { kind = "ClusterVirtualImage", name = "ubuntu-26-04-minimal-lts" }
            size         = "50Gi"
            storageClass = "replicated"
          }
          virtualMachine = {
            bootloader              = "EFI"
            cpu                     = { coreFraction = "50%", cores = 4 }
            memory                  = { size = "4Gi" }
            virtualMachineClassName = "amd-epyc-gen-3"
          }
        }
      }
    }

    secrets = {
      d8-credentials = {
        apiVersion = "v1"
        kind       = "Secret"
        metadata = {
          name      = "d8-credentials"
          namespace = "d8-cloud-provider-dvp"
        }
        stringData = {
          authScheme = "kubeconfig"
          secret     = "REDACTED"
        }
        type = "cloud-provider.deckhouse.io/credentials"
      }
    }

    settings = {
      apiVersion = "deckhouse.io/v1alpha1"
      kind       = "ModuleConfig"
      metadata   = { creationTimestamp = null, name = "cloud-provider-dvp" }
      spec       = { enabled = true, version = 1 }
    }
  }

  assert {
    condition     = lookup(output.nodeGroups, "master", null) != null
    error_message = "expected nodeGroups[master] to be present in passthrough mode"
  }

  assert {
    condition     = lookup(output.nodeGroups, "worker", null) != null
    error_message = "expected nodeGroups[worker] to be present in passthrough mode"
  }

  assert {
    condition     = lookup(output.instanceClasses, "master-dvp", null) != null
    error_message = "expected instanceClasses[master-dvp] to be present in passthrough mode"
  }

  assert {
    condition     = nonsensitive(lookup(output.secrets, "d8-credentials", null)) != null
    error_message = "expected secrets[d8-credentials] to be present in passthrough mode"
  }

  assert {
    condition     = output.settings.spec.version == 1
    error_message = "expected settings.spec.version == 1 in passthrough mode"
  }
}

# Scenario 2: migration in progress — PCC present, new resources absent, settings absent.
# use_pcc = true (has_pcc=true, new_resources_complete=false), synthesis mode.
run "with_pcc_migration_in_progress" {
  command = plan

  variables {
    providerClusterConfiguration = {
      apiVersion = "deckhouse.io/v1"
      kind       = "DVPClusterConfiguration"
      layout     = "Standard"
      masterNodeGroup = {
        instanceClass = {
          etcdDisk = { size = "5Gi", storageClass = "replicated" }
          rootDisk = {
            image        = { kind = "ClusterVirtualImage", name = "ubuntu-24-04-lts" }
            size         = "50Gi"
            storageClass = "replicated"
          }
          virtualMachine = {
            bootloader              = "EFI"
            cpu                     = { coreFraction = "20%", cores = 4 }
            ipAddresses             = ["Auto"]
            liveMigrationPolicy     = "PreferForced"
            memory                  = { size = "8Gi" }
            runPolicy               = "AlwaysOnUnlessStoppedManually"
            virtualMachineClassName = "amd-epyc-gen-3"
          }
        }
        replicas = 1
      }
      nodeGroups = [
        {
          instanceClass = {
            rootDisk = {
              image = { kind = "ClusterVirtualImage", name = "ubuntu-24-04-lts" }
              size  = "40Gi"
            }
            virtualMachine = {
              bootloader              = "EFI"
              cpu                     = { coreFraction = "20%", cores = 4 }
              liveMigrationPolicy     = "PreferForced"
              memory                  = { size = "4Gi" }
              runPolicy               = "AlwaysOnUnlessStoppedManually"
              virtualMachineClassName = "amd-epyc-gen-3"
            }
          }
          name     = "worker"
          replicas = 1
        }
      ]
      provider = {
        kubeconfigDataBase64 = "REDACTED"
        namespace            = "team-d8-cloud-providers"
        networkPolicy        = "Isolated"
      }
      sshPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIIZakrNbKZ7i/uDQqxy7/FtPr4+H+pT7VC7ZxdVp0QXA"
    }

    nodeGroups      = {}
    instanceClasses = {}
    secrets         = {}
    settings        = null
  }

  assert {
    condition     = lookup(output.nodeGroups, "master", null) != null
    error_message = "expected nodeGroups[master] synthesised from PCC"
  }

  assert {
    condition     = lookup(output.nodeGroups, "worker", null) != null
    error_message = "expected nodeGroups[worker] synthesised from PCC nodeGroups list"
  }

  assert {
    condition     = lookup(output.instanceClasses, "master-fc613b4dfd67", null) != null
    error_message = "expected instanceClasses[master-fc613b4dfd67] synthesised from PCC masterNodeGroup"
  }

  assert {
    condition     = lookup(output.instanceClasses, "worker-87eba76e7f31", null) != null
    error_message = "expected instanceClasses[worker-87eba76e7f31] synthesised from PCC nodeGroups[0]"
  }

  assert {
    condition     = output.nodeGroups.master.spec.cloudInstances.classReference.name == "master-fc613b4dfd67"
    error_message = "expected synthesised master NodeGroup to reference hashed DVPInstanceClass name"
  }

  assert {
    condition     = output.nodeGroups.worker.spec.cloudInstances.classReference.name == "worker-87eba76e7f31"
    error_message = "expected synthesised worker NodeGroup to reference hashed DVPInstanceClass name"
  }

  assert {
    condition     = output.instanceClasses["master-fc613b4dfd67"].metadata.name == "master-fc613b4dfd67"
    error_message = "expected synthesised master DVPInstanceClass metadata.name to match hashed map key"
  }

  assert {
    condition     = output.instanceClasses["worker-87eba76e7f31"].metadata.name == "worker-87eba76e7f31"
    error_message = "expected synthesised worker DVPInstanceClass metadata.name to match hashed map key"
  }

  assert {
    condition     = nonsensitive(lookup(output.secrets, "d8-credentials", null)) != null
    error_message = "expected secrets[d8-credentials] synthesised from PCC provider"
  }

  assert {
    condition     = output.settings.spec.version == 2
    error_message = "expected synthesised settings.spec.version == 2"
  }
}

# Scenario 3: destroy state — PCC absent, all maps empty, settings absent.
# use_pcc = false, passthrough mode with empty inputs; no crash expected.
run "destroy_empty_maps" {
  command = plan

  variables {
    providerClusterConfiguration = null
    nodeGroups                   = {}
    instanceClasses              = {}
    secrets                      = {}
    settings                     = null
  }

  assert {
    condition     = lookup(output.nodeGroups, "master", null) == null
    error_message = "expected no nodeGroups[master] when input maps are empty"
  }

  assert {
    condition     = lookup(output.instanceClasses, "master-fc613b4dfd67", null) == null
    error_message = "expected no instanceClasses[master-fc613b4dfd67] when input maps are empty"
  }

  assert {
    condition     = nonsensitive(lookup(output.secrets, "d8-credentials", null)) == null
    error_message = "expected no secrets[d8-credentials] when input maps are empty"
  }
}
