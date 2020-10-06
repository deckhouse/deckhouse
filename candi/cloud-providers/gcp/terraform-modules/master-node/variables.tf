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

locals {
  prefix                       = var.clusterConfiguration.cloud.prefix
  machine_type                 = var.providerClusterConfiguration.masterNodeGroup.instanceClass.machineType
  image                        = var.providerClusterConfiguration.masterNodeGroup.instanceClass.image
  disk_size_gb                 = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskSizeGb", 20)
  ssh_key                      = var.providerClusterConfiguration.sshKey
  ssh_user                     = "user"
  disable_external_ip          = var.providerClusterConfiguration.layout == "WithoutNAT" ? false : lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "disableExternalIP", true)
  configured_zones             = lookup(var.providerClusterConfiguration.masterNodeGroup, "zones", [])
  additional_network_tags      = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalNetworkTags", [])
  service_account_client_email = jsondecode(var.providerClusterConfiguration.provider.serviceAccountJSON).client_email
  additional_labels            = merge(lookup(var.providerClusterConfiguration, "labels", {}), lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalLabels", null))
}
