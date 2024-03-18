# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "node_name_prefix" {
  description = "Prefix for Node naming"
  default = "d8"
}

variable "nodeIndex" {
  type    = number
  default = 0
}

locals {
  resource_name_prefix = var.clusterConfiguration.cloud.prefix
  vnic_profile_id = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "vnicProfileId", [])
  storage_domain_id = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "storageDomainId", [])
  cluster_id = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "clusterId", [])
  template_name = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "template", [])
  master_node_name = join("-", [local.resource_name_prefix, "master", var.nodeIndex])
  master_cpus = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "numCPUs", [])
  master_ram_mb = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "memory", [])
  master_vm_type = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "vmType", [])
  master_nic_name = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "nicName", [])
  ssh_pubkey = lookup(var.providerClusterConfiguration, "sshPublicKey", null)
  master_root_disk_size = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "rootDiskSizeGb", 20)*1024*1024*1024
  master_etcd_disk_size = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "etcdDiskSizeGb", 10)*1024*1024*1024

  master_cloud_init_script = yamlencode({
    "hostname": local.master_node_name,
    "create_hostname_file": true,
    "ssh_deletekeys": true,
    "ssh_genkeytypes": ["rsa", "ecdsa", "ed25519"],
    "ssh_authorized_keys" : [local.ssh_pubkey]
  })
}
