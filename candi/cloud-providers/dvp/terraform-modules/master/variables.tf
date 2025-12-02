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

variable "api_version" {
  default = "virtualization.deckhouse.io/v1alpha2"
  type    = string
}

variable "prefix" {
  type = string
}

variable "cluster_uuid" {
  type = string
}

variable "node_group" {
  type = string
}

variable "namespace" {
  type = string
}

variable "node_index" {
  type = string
}

variable "hostname" {
  type = string
}

variable "additional_labels" {
  default = {}
  type    = map(string)
}

variable "additional_annotations" {
  default = {}
  type    = map(string)
}

variable "priority_class_name" {
  default = null
  type    = string
}

variable "node_selector" {
  default = null
  type    = map(string)
}

variable "tolerations" {
  default = null
  type    = list(map(string))
}

variable "zone" {
  default = ""
  type    = string
}

variable "region" {
  default = ""
  type    = string
}

variable "root_disk" {
  type = object({
    name = string
    hash = string
  })
}

variable "kubernetes_data_disk" {
  type = object({
    name   = string
    hash   = string
    md5_id = string
  })
}

variable "additional_disks" {
  type = list(object({
    name          = string
    hash          = string
    md5_id        = string
  }))
  default = []
}

variable "cpu" {
  type = object({
    cores         = number
    core_fraction = string
  })
}

variable "memory_size" {
  type = string
}

variable "bootloader" {
  type    = string
  default = "BIOS"
}

variable "ipv4_address" {
  default = null
  type = object({
    name    = string
    address = string
  })
}

variable "cloud_config" {
  default = ""
  type    = string
}

variable "ssh_public_key" {
  type = string
}

variable "virtual_machine_class_name" {
  type    = string
  default = "generic"
}

variable "timeouts" {
  default = { "create" = "30m", "update" = "5m", "delete" = "5m" }
  type = object({
    create = string
    update = string
    delete = string
  })
}

locals {
  vm_merged_node_selector = merge(
    {
      for k, v in {
        "topology.kubernetes.io/zone"   = var.zone,
        "topology.kubernetes.io/region" = var.region,
      } : k => v if v != ""
    },
    var.node_selector
  )
#
  vm_destructive_params = merge(
    {
      "virtualMachine" = {
        "cpu" = {
          "cores"        = var.cpu.cores
          "coreFraction" = var.cpu.core_fraction
        }
        "memory" = {
          "size" = var.memory_size
        }
        "nodeSelector"      = local.vm_merged_node_selector
        "tolerations"       = var.tolerations
        "priorityClassName" = var.priority_class_name
        "ipAddress"         = var.ipv4_address.address
        "sshPublicKeyHash"  = sha256(jsonencode(var.ssh_public_key))
      }
    },
    {
      "rootDiskHash" = var.root_disk.hash,
      "etcDiskHash"  = var.kubernetes_data_disk.hash
      "additionalDisksHash" = local.additional_disks_hashes
    },
  )

  vm_destructive_params_json      = jsonencode(local.vm_destructive_params)
  vm_destructive_params_json_hash = substr(sha256(jsonencode(local.vm_destructive_params)), 0, 6)

  vm_name               = join("-", [var.hostname, local.vm_destructive_params_json_hash])
  cloudinit_secret_name = join("-", [var.hostname, "cloudinit", local.vm_destructive_params_json_hash])

  vm_merged_labels = merge(
    {
      "dvp.deckhouse.io/cluster-prefix" = var.prefix
      "dvp.deckhouse.io/cluster-uuid"   = var.cluster_uuid
      "dvp.deckhouse.io/node-group"     = var.node_group
      "dvp.deckhouse.io/hostname"       = var.hostname
    },
    var.additional_labels
  )

  vm_merged_annotations = merge(
    {
      "last_applied_destructive_vm_parameters"      = local.vm_destructive_params_json
      "last_applied_destructive_vm_parameters_hash" = local.vm_destructive_params_json_hash
    },
    var.additional_annotations
  )
}
