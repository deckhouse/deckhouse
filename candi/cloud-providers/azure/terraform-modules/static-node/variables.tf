variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
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

variable "nodeGroupName" {
  type = string
}

locals {
  prefix             = var.clusterConfiguration.cloud.prefix
  admin_username     = "azureuser"
  ssh_public_key     = var.providerClusterConfiguration.sshPublicKey
  node_groups        = lookup(var.providerClusterConfiguration, "nodeGroups", [])
  node_group         = [for i in local.node_groups : i if i.name == var.nodeGroupName][0]
  node_group_name    = local.node_group.name
  machine_size       = local.node_group.instanceClass.machineSize
  disk_type          = lookup(local.node_group.instanceClass, "diskType", "StandardSSD_LRS")
  disk_size_gb       = lookup(local.node_group.instanceClass, "diskSizeGb", 50)
  enable_external_ip = lookup(local.node_group.instanceClass, "enableExternalIP", false)
  urn                = split(":", local.node_group.instanceClass.urn)
  image_publisher    = local.urn[0]
  image_offer        = local.urn[1]
  image_sku          = local.urn[2]
  image_version      = local.urn[3]
  zones              = lookup(local.node_group, "zones", ["1", "2", "3"])
  additional_tags    = merge(lookup(var.providerClusterConfiguration, "tags", {}), lookup(local.node_group.instanceClass, "additionalTags", {}))
}
