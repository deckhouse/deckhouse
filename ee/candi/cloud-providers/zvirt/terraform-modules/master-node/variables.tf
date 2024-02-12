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
  master_cpus = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "numCPUs", [])
  master_ram_mb = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "memory", [])
  master_os_type = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "os", [])
  master_vm_type = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "vmType", [])
  master_nic_name = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "nicName", [])
  ssh_pubkey = lookup(var.providerClusterConfiguration, "sshPublicKey", null)

  master_cloud_init_script = yamlencode({
    "ssh_keys": {
      "ssh_authorized_keys": local.ssh_pubkey,
    }
  })
}