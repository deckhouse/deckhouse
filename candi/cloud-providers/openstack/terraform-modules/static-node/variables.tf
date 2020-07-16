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
  type = string
}

variable "cloudConfig" {
  type = string
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  pod_subnet_cidr = var.clusterConfiguration.podSubnetCIDR
  ng = [for i in var.providerClusterConfiguration.nodeGroups: i if i.name == var.nodeGroupName][0]
  instance_class = local.ng["instanceClass"]
  flavor_name = local.instance_class["flavorName"]
  image_name = local.instance_class["imageName"]
  root_disk_size = lookup(local.instance_class, "rootDiskSizeInGb", "")
  config_drive = lookup(local.instance_class, "configDrive", false)
  networks = local.instance_class["networks"]
  floating_ip_pools = lookup(local.instance_class, "floatingIpPools", [])
  security_group_names = lookup(local.instance_class, "securityGroups", [])
}
