# Copyright 2024 Flant JSC
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
  apiVersion = "virtualization.deckhouse.io/v1alpha2"

  vm_merged_node_selector = merge(
    {
      for k, v in {
        "topology.kubernetes.io/zone"   = local.zone,
        "topology.kubernetes.io/region" = local.region,
      } : k => v if v != ""
    },
    local.vm_node_selector,
  )

  root_disk_destructive_params = {
    "rootDisk" = {
      "storageClassName" = local.root_disk_storage_class_name
      "image" = {
        "type" = local.root_disk_image_type
        "name" = local.root_disk_image_name
      }
    }
  }

  etc_disk_destructive_params = {
    "rootDisk" = {
      "storageClassName" = local.etcd_disk_storage_class_name
    }
  }

  vm_destructive_params = merge({
    "virtualMachine" = {
      "cpu" = {
        "cores"        = local.vm_cpu_cores
        "coreFraction" = local.vm_cpu_core_fraction
      }
      "memory" = {
        "size" = local.vm_memory_size
      }
      "nodeSelector"      = local.vm_merged_node_selector
      "tolerations"       = local.vm_tolerations
      "priorityClassName" = local.vm_priority_class_name
      "ipAddress"         = local.vm_ip_address
      "sshPublicKeyHash"  = sha256(jsonencode(local.ssh_public_key))
      "cloudConfigHash"   = sha256(jsonencode(var.cloudConfig))
    }
    },
    {
      "rootDiskHash" = local.root_disk_destructive_params_json_hash,
      "etcDiskHash"  = local.etc_disk_destructive_params_json_hash
    },
  )

  root_disk_destructive_params_json      = jsonencode(local.root_disk_destructive_params)
  root_disk_destructive_params_json_hash = substr(sha256(jsonencode(local.root_disk_destructive_params_json)), 0, 6)

  etc_disk_destructive_params_json      = jsonencode(local.etc_disk_destructive_params)
  etc_disk_destructive_params_json_hash = substr(sha256(jsonencode(local.etc_disk_destructive_params_json)), 0, 6)

  vm_destructive_params_json      = jsonencode(local.vm_destructive_params)
  vm_destructive_params_json_hash = substr(sha256(jsonencode(local.vm_destructive_params)), 0, 6)

  root_disk_name        = join("-", [local.prefix, "master-root", var.nodeIndex, local.root_disk_destructive_params_json_hash])
  etc_disk_name         = join("-", [local.prefix, "kubernetes-data", var.nodeIndex, local.etc_disk_destructive_params_json_hash])
  vm_host_name          = join("-", [local.prefix, "master", var.nodeIndex])
  vm_name               = join("-", [local.vm_host_name, local.vm_destructive_params_json_hash])
  cloudinit_secret_name = join("-", [local.vm_host_name, "cloudinit", local.vm_destructive_params_json_hash])
  vmip_name             = format("%s-%s", local.vm_host_name, replace(local.vm_ip_address, ".", "-"))

  vm_merged_labels = merge(
    {
      "dvp.deckhouse.io/cluster-prefix" = local.prefix
      "dvp.deckhouse.io/cluster-uuid"   = var.clusterUUID
      "dvp.deckhouse.io/node-group"     = "master"
    },
  local.vm_additional_labels)

  vm_merged_annotations = merge(
    {
      "last_applied_destructive_vm_parameters"      = local.vm_destructive_params_json
      "last_applied_destructive_vm_parameters_hash" = local.vm_destructive_params_json_hash
    },
    local.vm_additional_annotations
  )

  root_disk_annotations = {
    "last_applied_destructive_root_disk_parameters"      = local.root_disk_destructive_params_json
    "last_applied_destructive_root_disk_parameters_hash" = local.root_disk_destructive_params_json_hash
  }

  etc_disk_annotations = {
    "last_applied_destructive_root_disk_parameters"      = local.etc_disk_destructive_params_json
    "last_applied_destructive_root_disk_parameters_hash" = local.etc_disk_destructive_params_json_hash
  }
}

resource "kubernetes_manifest" "master-root-disk" {
  manifest = {
    "apiVersion" = local.apiVersion
    "kind"       = "VirtualMachineDisk"
    "metadata" = {
      "name"        = local.root_disk_name
      "namespace"   = local.namespace
      "annotations" = local.root_disk_annotations
    }
    "spec" = {
      "dataSource" = merge(
        { "type" = local.root_disk_image_type },
        local.root_disk_image_type == "ClusterVirtualMachineImage" ? { "clusterVirtualMachineImage" = { "name" = local.root_disk_image_name } } : {},
        local.root_disk_image_type == "VirtualMachineImage" ? { "virtualMachineImage" = { "name" = local.root_disk_image_name } } : {}
      )
      "persistentVolumeClaim" = merge({
        "size" = local.root_disk_size
        },
        local.root_disk_storage_class_name != null ? { "storageClassName" = local.root_disk_storage_class_name } : null
      )
    }
  }
  wait {
    fields = {
      "status.phase" = "Ready"
    }
  }
  timeouts {
    create = "30m"
    update = "1m"
    delete = "1m"
  }
  lifecycle {
    ignore_changes = [
      object.spec.persistentVolumeClaim.storageClassName
    ]
  }
}

resource "kubernetes_manifest" "kubernetes-data-disk" {
  manifest = {
    "apiVersion" = local.apiVersion
    "kind"       = "VirtualMachineDisk"
    "metadata" = {
      "name"        = local.etc_disk_name
      "namespace"   = local.namespace
      "annotations" = local.etc_disk_annotations
    }
    "spec" = {
      "persistentVolumeClaim" = merge({
        "size" = local.etcd_disk_size
        },
        local.etcd_disk_storage_class_name != null ? { "storageClassName" = local.etcd_disk_storage_class_name } : null
      )
    }
  }
  wait {
    fields = {
      "status.phase" = "Ready"
    }
  }
  timeouts {
    create = "30m"
    update = "5m"
    delete = "5m"
  }
}

resource "kubernetes_manifest" "vmip" {
  count = local.vm_ip_address != "" ? 1 : 0
  manifest = {
    "apiVersion" = local.apiVersion
    "kind"       = "VirtualMachineIPAddressClaim"
    "metadata" = {
      "name"      = local.vmip_name
      "namespace" = local.namespace
    }
    "spec" = {
      "address"       = local.vm_ip_address
      "reclaimPolicy" = "Delete"
    }
  }
  wait {
    fields = {
      "status.phase" = "Bound"
    }
  }
  timeouts {
    create = "10m"
    update = "5m"
    delete = "5m"
  }
}

resource "kubernetes_secret" "master-cloudinit-secret" {
  metadata {
    name      = local.cloudinit_secret_name
    namespace = local.namespace
  }
  data = {
    userData = templatefile("${path.module}/templates/cloudinit.tftpl", {
      host_name      = local.vm_host_name
      ssh_public_key = local.ssh_public_key
      user_data      = var.cloudConfig == "" ? "" : base64decode(var.cloudConfig)
    })
  }
  type = "Opaque"
}

resource "kubernetes_manifest" "vm" {

  field_manager {
    force_conflicts = true
  }

  manifest = {
    "apiVersion" = local.apiVersion
    "kind"       = "VirtualMachine"
    "metadata" = {
      "annotations" = local.vm_merged_annotations
      "labels"      = local.vm_merged_labels
      "name"        = local.vm_name
      "namespace"   = local.namespace
    }
    "spec" = merge(
      {
        "terminationGracePeriodSeconds" = 90

        "bootloader"               = "BIOS"
        "enableParavirtualization" = true
        "osType"                   = "Generic"
        "runPolicy"                = "AlwaysOn"

        "disruptions" = {
          "restartApprovalMode" = "Automatic"
        }

        "cpu" = {
          "coreFraction" = local.vm_cpu_core_fraction
          "cores"        = local.vm_cpu_cores
        }

        "memory" = {
          "size" = local.vm_memory_size
        }

        "blockDevices" = [
          {
            "type" = "VirtualMachineDisk"
            "virtualMachineDisk" = {
              "name" = kubernetes_manifest.master-root-disk.manifest.metadata.name
            }
          },
          {
            "type" = "VirtualMachineDisk"
            "virtualMachineDisk" = {
              "name" = kubernetes_manifest.kubernetes-data-disk.manifest.metadata.name
            }
          },
        ]

        "provisioning" = {
          "type" = "UserDataSecret"
          "userDataSecretRef" = {
            "name" = local.cloudinit_secret_name
          }
        }

      },
      local.vm_ip_address != "" ? { "virtualMachineIPAddressClaimName" = kubernetes_manifest.vmip[0].manifest.metadata.name } : null,
      local.vm_priority_class_name != null ? { "priorityClassName" = local.vm_priority_class_name } : null,
      local.vm_tolerations != null ? { "tolerations" = local.vm_tolerations } : null,
      length(local.vm_merged_node_selector) != 0 ? { "nodeSelector" = local.vm_merged_node_selector } : null,
    )
  }

  wait {
    fields = {
      "status.phase" = "Running",
    }
  }

  timeouts {
    create = "30m"
    update = "5m"
    delete = "5m"
  }
}
