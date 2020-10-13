variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
  validation {
    condition     = contains(keys(var.providerClusterConfiguration), "subnetworkCIDR") ? cidrsubnet(var.providerClusterConfiguration.subnetworkCIDR, 0, 0) == var.providerClusterConfiguration.subnetworkCIDR : true
    error_message = "Invalid subnetworkCIDR in GCPClusterConfiguration."
  }
}

variable "nodeIndex" {
  type    = string
  default = ""
}

variable "cloudConfig" {
  type    = string
  default = ""
}

variable "nodeGroupName" {
  type = string
}

variable "clusterUUID" {
  type = string
}

locals {
  prefix                       = var.clusterConfiguration.cloud.prefix
  node_groups                  = lookup(var.providerClusterConfiguration, "nodeGroups", [])
  node_group                   = [for i in local.node_groups : i if i.name == var.nodeGroupName][0]
  node_group_name              = local.node_group.name
  machine_type                 = local.node_group.instanceClass.machineType
  image                        = local.node_group.instanceClass.image
  disk_size_gb                 = lookup(local.node_group.instanceClass, "diskSizeGb", 20)
  disk_type                    = lookup(local.node_group.instanceClass, "diskType", "pd-ssd")
  ssh_key                      = var.providerClusterConfiguration.sshKey
  ssh_user                     = "user"
  disable_external_ip          = var.providerClusterConfiguration.layout == "WithoutNAT" ? false : lookup(local.node_group.instanceClass, "disableExternalIP", true)
  configured_zones             = lookup(local.node_group, "zones", [])
  additional_network_tags      = lookup(local.node_group.instanceClass, "additionalNetworkTags", [])
  service_account_client_email = jsondecode(var.providerClusterConfiguration.provider.serviceAccountJSON).client_email
  additional_labels            = merge(lookup(var.providerClusterConfiguration, "labels", {}), lookup(local.node_group.instanceClass, "additionalLabels", {}))
}
