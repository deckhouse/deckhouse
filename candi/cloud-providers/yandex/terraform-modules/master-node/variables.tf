variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "nodeIndex" {
  type = number
  default = 0
}

variable "cloudConfig" {
  type = string
  default = ""
}

variable "clusterUUID" {
  type = string
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  mng = var.providerClusterConfiguration.masterNodeGroup
  master_instance_class = var.providerClusterConfiguration.masterNodeGroup.instanceClass
  cores = local.master_instance_class.cores
  memory = local.master_instance_class.memory / 1024
  disk_size_gb = lookup(local.master_instance_class, "diskSizeGb", 20)
  image_id = local.master_instance_class.imageID
  ssh_public_key = var.providerClusterConfiguration.sshPublicKey
  external_ip_addresses = lookup(local.master_instance_class, "externalIPAddresses", [])
  external_subnet_id = lookup(local.master_instance_class, "externalSubnetID", null)

  additional_labels = merge(lookup(var.providerClusterConfiguration, "labels", {}), lookup(local.master_instance_class, "additionalLabels", {}))
}
