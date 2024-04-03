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
  type = string
  default = ""
}

variable "nodeGroupName" {
  type = string
}

locals {
  resource_name_prefix = var.clusterConfiguration.cloud.prefix
  ng             = [for i in var.providerClusterConfiguration.nodeGroups : i if i.name == var.nodeGroupName][0]
  instance_class = local.ng["instanceClass"]
  node_group_name = local.ng.name

  vnic_profile_id = lookup(local.instance_class, "vnicProfileID", [])
  cluster_id = lookup(var.providerClusterConfiguration, "clusterID", [])
  template_name = lookup(local.instance_class, "template", [])
  node_name = join("-", [local.resource_name_prefix, local.node_group_name, var.nodeIndex])
  cpus = lookup(local.instance_class, "numCPUs", [])
  ram_mb = lookup(local.instance_class, "memory", [])
  vm_type = "high_performance"
  nic_name = "nic1"
  ssh_pubkey = lookup(var.providerClusterConfiguration, "sshPublicKey", null)
  root_disk_size = lookup(local.instance_class, "rootDiskSizeGb", 20)*1024*1024*1024

  cloud_init_script = yamlencode(merge({
    "hostname": local.node_name,
    "create_hostname_file": true,
    "ssh_deletekeys": true,
    "ssh_genkeytypes": ["rsa", "ecdsa", "ed25519"],
    "ssh_authorized_keys" : [local.ssh_pubkey]
  }, length(var.cloudConfig) > 0 ? yamldecode(base64decode(var.cloudConfig)) : tomap({})))
}
