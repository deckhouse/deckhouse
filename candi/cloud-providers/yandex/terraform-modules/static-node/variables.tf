variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "nodeGroupName" {
  type = string
}

variable "nodeIndex" {
  type = number
}

variable "cloudConfig" {
  type = string
  default = ""
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  ng = [for i in var.providerClusterConfiguration.nodeGroups: i if i.name == var.nodeGroupName][0]
  instance_class = local.ng["instanceClass"]
  cores = local.instance_class.cores
  core_fraction = lookup(local.instance_class, "coreFraction", null)
  memory = local.instance_class.memory / 1024
  disk_size_gb = lookup(local.instance_class, "diskSizeGb", 20)
  image_id = local.instance_class.imageID
  ssh_public_key = var.providerClusterConfiguration.sshPublicKey
  external_ip_addresses = lookup(local.instance_class, "externalIPAddresses", [])
  external_subnet_id = lookup(local.instance_class, "externalSubnetID", null)

  additional_labels = merge(lookup(var.providerClusterConfiguration, "labels", {}), lookup(local.master_instance_class, "additionalLabels", {}))
}
