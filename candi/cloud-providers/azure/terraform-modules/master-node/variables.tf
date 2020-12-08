variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any

  validation {
    condition     = replace(var.providerClusterConfiguration.masterNodeGroup.instanceClass.urn, "latest", "") == var.providerClusterConfiguration.masterNodeGroup.instanceClass.urn ? true : false
    error_message = "Not allowed to use latest as image version."
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

variable "clusterUUID" {
  type = string
}

locals {
  prefix             = var.clusterConfiguration.cloud.prefix
  admin_username     = "azureuser"
  machine_size       = var.providerClusterConfiguration.masterNodeGroup.instanceClass.machineSize
  ssh_public_key     = var.providerClusterConfiguration.sshPublicKey
  disk_type          = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskType", "StandardSSD_LRS")
  disk_size_gb       = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskSizeGb", 50)
  enable_external_ip = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "enableExternalIP", false)
  urn                = split(":", var.providerClusterConfiguration.masterNodeGroup.instanceClass.urn)
  image_publisher    = local.urn[0]
  image_offer        = local.urn[1]
  image_sku          = local.urn[2]
  image_version      = local.urn[3]
  zones              = lookup(var.providerClusterConfiguration.masterNodeGroup, "zones", ["1", "2", "3"])
  additional_tags    = merge(lookup(var.providerClusterConfiguration, "tags", {}), lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalTags", {}))
}
