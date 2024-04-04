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

locals {
  resource_name_prefix = var.clusterConfiguration.cloud.prefix
  vnic_profile_id = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "vnicProfileID", [])
  storage_domain_id = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "storageDomainID", [])
  cluster_id = lookup(var.providerClusterConfiguration, "clusterID", [])
  template_name = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "template", [])
  master_node_name = join("-", [local.resource_name_prefix, "master", var.nodeIndex])
  master_cpus = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "numCPUs", [])
  master_ram_mb = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "memory", [])
  master_vm_type = "high_performance"
  master_nic_name = "nic1"
  ssh_pubkey = lookup(var.providerClusterConfiguration, "sshPublicKey", null)
  master_root_disk_size = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "rootDiskSizeGb", 20)*1024*1024*1024
  master_etcd_disk_size = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "etcdDiskSizeGb", 10)*1024*1024*1024

  master_cloud_init_script = yamlencode(merge({
    "hostname": local.master_node_name,
    "create_hostname_file": true,
    "ssh_deletekeys": true,
    "ssh_genkeytypes": ["rsa", "ecdsa", "ed25519"],
    "ssh_authorized_keys" : [local.ssh_pubkey]
  }, length(var.cloudConfig) > 0 ? yamldecode(base64decode(var.cloudConfig)) : tomap({})))
}
