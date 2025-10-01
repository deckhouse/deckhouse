# Copyright 2025 Flant JSC
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

resource "kubernetes_secret" "cloudinit-secret" {
  metadata {
    name      = local.cloudinit_secret_name
    namespace = var.namespace
  }
  data = {
    userData = templatefile("${path.module}/templates/cloudinit.tftpl", {
      host_name      = var.hostname
      ssh_public_key = var.ssh_public_key
      user_data      = var.cloud_config == "" ? "" : base64decode(var.cloud_config)
    })
  }
  type = "provisioning.virtualization.deckhouse.io/cloud-init"
  lifecycle {
    ignore_changes = [
      data
    ]
  }
}

locals {
  additional_block_refs = tolist([
    for d in var.additional_disks : {
      "kind" = "VirtualDisk"
      "name" = d.name
    }
  ])

  additional_disks_hashes = [
    for d in var.additional_disks : d.hash
  ]

  spec = merge(
    {
      "terminationGracePeriodSeconds" = 90

      "bootloader"               = var.bootloader
      "enableParavirtualization" = true
      "osType"                   = "Generic"
      "runPolicy"                = "AlwaysOn"
      "virtualMachineClassName"  = var.virtual_machine_class_name

      "disruptions" = {
        "restartApprovalMode" = "Automatic"
      }

      "cpu" = {
        "coreFraction" = var.cpu.core_fraction
        "cores"        = var.cpu.cores
      }

      "memory" = {
        "size" = var.memory_size
      }

      "blockDeviceRefs" = concat(
        [
        {
          "kind" = "VirtualDisk"
          "name" = var.root_disk.name
        }
        ],
        local.additional_block_refs
      )

      "provisioning" = {
        "type" = "UserDataRef"
        "userDataRef" = {
          "kind" = "Secret"
          "name" = local.cloudinit_secret_name
        }
      }
    },
    var.ipv4_address != null && var.ipv4_address.name != "" ? { "virtualMachineIPAddressName" = var.ipv4_address.name } : null,
    var.priority_class_name != null ? { "priorityClassName" = var.priority_class_name } : null,
    var.tolerations != null ? { "tolerations" = var.tolerations } : null,
    length(local.vm_merged_node_selector) != 0 ? { "nodeSelector" = local.vm_merged_node_selector } : null,
  )

}

resource "kubernetes_manifest" "vm" {

  field_manager {
    force_conflicts = true
  }

  manifest = {
    "apiVersion" = var.api_version
    "kind"       = "VirtualMachine"
    "metadata" = {
      "annotations" = local.vm_merged_annotations
      "labels"      = local.vm_merged_labels
      "name"        = local.vm_name
      "namespace"   = var.namespace
    }
    "spec" = local.spec
  }

  wait {
    fields = {
      "status.phase" = "Running",
    }
  }

  timeouts {
    create = var.timeouts.create
    update = var.timeouts.update
    delete = var.timeouts.delete
  }
}

data "kubernetes_resource" "vm_data" {
  api_version = var.api_version
  kind        = "VirtualMachine"

  metadata {
    name      = local.vm_name
    namespace = var.namespace
  }
  depends_on = [
    kubernetes_manifest.vm
  ]
}
