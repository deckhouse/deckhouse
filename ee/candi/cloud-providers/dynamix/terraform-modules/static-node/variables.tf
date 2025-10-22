# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "nodeIndex" {
  type    = number
  default = 0
}

variable "cloudConfig" {
  type    = string
  default = ""
}

variable "nodeGroupName" {
  type = string
}

variable "resourceManagementTimeout" {
  type    = string
  default = "10m"
}

locals {
  resource_name_prefix = var.clusterConfiguration.cloud.prefix
  ng                   = [for i in var.providerClusterConfiguration.nodeGroups : i if i.name == var.nodeGroupName][0]
  instance_class       = local.ng["instanceClass"]
  node_group_name      = local.ng.name
  node_name            = join("-", [local.resource_name_prefix, local.node_group_name, var.nodeIndex])
  cpus                 = lookup(local.instance_class, "numCPUs", [])
  ram_mb               = lookup(local.instance_class, "memory", [])
  ssh_pubkey           = lookup(var.providerClusterConfiguration, "sshPublicKey", null)
  root_disk_size       = lookup(local.instance_class, "rootDiskSizeGb", 50)
  image_name           = lookup(local.instance_class, "imageName", null)
  resource_group_name  = join("-", [local.resource_name_prefix, "rg"])
  pool                 = lookup(local.instance_class, "pool", null)
  extnet_name          = lookup(local.instance_class, "externalNetwork", null)
  vins_name            = join("-", [local.resource_name_prefix, "vins"])
  driver               = "KVM_X86"
  net_type_vins        = "VINS"
  net_type_extnet      = "EXTNET"

  cloud_init_script = jsonencode(merge({
    "hostname" : local.node_name,
    "create_hostname_file" : true,
    "ssh_deletekeys" : true,
    "ssh_genkeytypes" : ["rsa", "ecdsa", "ed25519"],
    "ssh_authorized_keys" : [local.ssh_pubkey],
    "users" : [
      {
        "name" : "user",
        "ssh_authorized_keys" : [local.ssh_pubkey]
        "groups" : "users, wheel",
        "sudo" : "ALL=(ALL) NOPASSWD:ALL"
      }
    ]
  }, length(var.cloudConfig) > 0 ? try(jsondecode(base64decode(var.cloudConfig)), yamldecode(base64decode(var.cloudConfig))) : tomap({})))
}
