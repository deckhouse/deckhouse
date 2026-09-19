# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type    = any
  default = null
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

variable "nodeGroups" {
  type    = any
  default = {}
}

variable "instanceClasses" {
  type    = any
  default = {}
}

variable "secrets" {
  type    = any
  default = {}
}

variable "settings" {
  type    = any
  default = null
}

module "migration" {
  source                       = "../migration"
  providerClusterConfiguration = var.providerClusterConfiguration
  nodeGroups                   = var.nodeGroups
  instanceClasses              = var.instanceClasses
  secrets                      = var.secrets
  settings                     = var.settings
}

locals {
  resource_name_prefix = var.clusterConfiguration.cloud.prefix

  _provider_params = try(module.migration.settings.spec.settings.provider.parameters, {})
  _node_params     = try(module.migration.settings.spec.settings.nodes.parameters, {})

  _node_group     = try(module.migration.nodeGroups["master"], {})
  _instance_class = try(module.migration.instanceClasses[local._node_group.spec.cloudInstances.classReference.name].spec, {})

  cluster_id = try(local._provider_params.clusterID, "")
  ssh_pubkey = try(local._node_params.sshPublicKey, null)

  vnic_profile_id   = try(local._instance_class.vnicProfileID, "")
  storage_domain_id = try(local._instance_class.storageDomainID, "")
  template_name     = try(local._instance_class.template, "")
  master_cpus       = try(local._instance_class.numCPUs, 0)
  master_ram_mb     = try(local._instance_class.memory, 0)

  master_node_name = join("-", [local.resource_name_prefix, "master", var.nodeIndex])
  master_vm_type   = "high_performance"
  master_nic_name  = "nic1"

  master_root_disk_size = try(local._instance_class.rootDiskSizeGb, 50) * 1024 * 1024 * 1024
  master_etcd_disk_size = try(local._instance_class.etcdDiskSizeGb, 10) * 1024 * 1024 * 1024

  _custom_network        = try(local._node_params.customNetworkConfigs["master"], null)
  custom_network_config  = local._custom_network == null ? [] : [1]
  custom_network_name    = try(local._custom_network.networkInterfaceName, "")
  custom_network_address = try(local._custom_network.networkInterfaceAddresses[var.nodeIndex], "")
  custom_network_netmask = try(local._custom_network.networkInterfaceNetmask, "")
  custom_network_gateway = try(local._custom_network.networkInterfaceGateway, "")
  custom_network_dns     = join(" ", try(tolist(local._custom_network.dnsServers), []))

  master_cloud_init_script = yamlencode(merge({
    "hostname" : local.master_node_name,
    "create_hostname_file" : true,
    "ssh_deletekeys" : true,
    "ssh_genkeytypes" : ["rsa", "ecdsa", "ed25519"],
    "ssh_authorized_keys" : [local.ssh_pubkey]
  }, length(var.cloudConfig) > 0 ? yamldecode(base64decode(var.cloudConfig)) : tomap({})))
}
