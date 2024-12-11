# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
  validation {
    condition     = cidrsubnet(var.providerClusterConfiguration.standard.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.standard.internalNetworkCIDR
    error_message = "Invalid internalNetworkCIDR in HuaweiCloudClusterConfiguration."
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

variable "nodeGroupName" {
  type = string
}

locals {
  prefix                = var.clusterConfiguration.cloud.prefix
  ng                    = [for i in var.providerClusterConfiguration.nodeGroups : i if i.name == var.nodeGroupName][0]
  instance_class        = local.ng["instanceClass"]
  standard              = lookup(var.providerClusterConfiguration, "standard", {})
  pod_subnet_cidr       = var.clusterConfiguration.podSubnetCIDR
  internal_network_cidr = var.providerClusterConfiguration.standard.internalNetworkCIDR
  network_security      = lookup(var.providerClusterConfiguration.standard, "internalNetworkSecurity", true)
  image_name            = local.instance_class["imageName"]
  tags                  = lookup(var.providerClusterConfiguration, "tags", {})
  ssh_allow_list        = lookup(var.providerClusterConfiguration, "sshAllowList", ["0.0.0.0/0"])
  server_group          = lookup(local.ng, "serverGroup", {})
  server_group_policy   = lookup(local.server_group, "policy", "")
  security_group_names  = local.network_security ? concat([local.prefix], lookup(local.instance_class, "additionalSecurityGroups", [])) : []
  volume_type_map       = local.ng["volumeTypeMap"]
  actual_zones          = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.huaweicloud_availability_zones.zones.names, var.providerClusterConfiguration.zones)) : data.huaweicloud_availability_zones.zones.names
  zone                  = element(tolist(setintersection(keys(local.volume_type_map), local.actual_zones)), var.nodeIndex)
  volume_type           = local.volume_type_map[local.zone]
  flavor_name           = local.instance_class["flavorName"]
  root_disk_size        = lookup(local.instance_class, "rootDiskSize", 50) # Huaweicloud can have disks predefined within vm flavours, so we do not set any defaults here
  additional_tags       = lookup(local.instance_class, "additionalTags", {})
}
