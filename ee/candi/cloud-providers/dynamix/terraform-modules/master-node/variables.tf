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

variable "resourceManagementTimeout" {
  type    = string
  default = "10m"
}

locals {
  resource_name_prefix      = var.clusterConfiguration.cloud.prefix
  master_node_name          = join("-", [local.resource_name_prefix, "master", var.nodeIndex])
  master_cpus               = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "numCPUs", [])
  master_ram_mb             = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "memory", [])
  ssh_pubkey                = lookup(var.providerClusterConfiguration, "sshPublicKey", null)
  master_root_disk_size     = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "rootDiskSizeGb", 50)
  master_etcd_disk_size     = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "etcdDiskSizeGb", 15)
  image_name                = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "imageName", null)
  resource_group_name       = join("-", [local.resource_name_prefix, "rg"])
  kubernetes_data_disk_name = join("-", [local.master_node_name, "kubernetes-data"])
  location                  = lookup(var.providerClusterConfiguration, "location", null)
  storage_endpoint          = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "storageEndpoint", null)
  pool                      = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "pool", null)
  extnet_name               = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "externalNetwork", null)
  vins_name                 = join("-", [local.resource_name_prefix, "vins"])
  driver                    = "KVM_X86"
  net_type_vins             = "VINS"
  net_type_extnet           = "EXTNET"

  master_cloud_init_script = jsonencode(merge({
    "hostname" : local.master_node_name,
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
