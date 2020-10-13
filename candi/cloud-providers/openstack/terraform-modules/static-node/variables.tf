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

variable "clusterUUID" {
  type = string
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  pod_subnet_cidr = var.clusterConfiguration.podSubnetCIDR
  ng = [for i in var.providerClusterConfiguration.nodeGroups: i if i.name == var.nodeGroupName][0]
  instance_class = local.ng["instanceClass"]
  flavor_name = local.instance_class["flavorName"]
  image_name = local.instance_class["imageName"]
  root_disk_size = lookup(local.instance_class, "rootDiskSize", "")
  config_drive = lookup(local.instance_class, "configDrive", false)
  networks = concat([local.instance_class["mainNetwork"]], lookup(local.instance_class, "additionalNetworks", []))
  networks_with_security_disabled = lookup(local.instance_class, "networksWithSecurityDisabled", [])
  floating_ip_pools = lookup(local.instance_class, "floatingIPPools", [])
  layout = join("", [lower(substr(var.providerClusterConfiguration.layout, 0, 1)), substr(var.providerClusterConfiguration.layout, 1, -1)])
  pod_network_mode = local.layout == "simple" ? lookup(var.providerClusterConfiguration.simple, "podNetworkMode", "VXLAN") : local.layout == "simpleWithInternalNetwork" ? lookup(var.providerClusterConfiguration.simpleWithInternalNetwork, "podNetworkMode", "DirectRoutingWithPortSecurityEnabled") : ""
  internal_network_security = (local.layout == "standard" || local.layout == "standardWithNoRouter") ? lookup(var.providerClusterConfiguration[local.layout], "internalNetworkSecurity", true) : false
  internal_network_security_enabled = local.pod_network_mode == "DirectRoutingWithPortSecurityEnabled" || local.internal_network_security
  additional_sg_names = lookup(local.instance_class, "additionalSecurityGroups", [])
  security_group_names = (local.layout == "standard" || local.layout == "standardWithNoRouter") ? concat([local.prefix], local.additional_sg_names) : local.additional_sg_names
  metadata_tags = merge(lookup(var.providerClusterConfiguration, "tags", {}), lookup(local.instance_class, "additionalTags", {}))
  zones = lookup(local.ng, "zones", null)
}
