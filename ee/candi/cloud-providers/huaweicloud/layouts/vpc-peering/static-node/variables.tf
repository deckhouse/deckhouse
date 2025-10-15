# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
  validation {
    condition     = cidrsubnet(var.providerClusterConfiguration.vpcPeering.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.vpcPeering.internalNetworkCIDR
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

variable "mainNetwork" {
  type        = string
  default     = null
}

variable "additionalNetworks" {
  type        = list(string)
  default     = []
}

locals {
  prefix                = var.clusterConfiguration.cloud.prefix
  ng                    = [for i in var.providerClusterConfiguration.nodeGroups : i if i.name == var.nodeGroupName][0]
  instance_class        = local.ng["instanceClass"]
  pod_subnet_cidr       = var.clusterConfiguration.podSubnetCIDR
  internal_network_cidr = var.providerClusterConfiguration.vpcPeering.internalNetworkCIDR
  network_security      = lookup(var.providerClusterConfiguration.vpcPeering, "internalNetworkSecurity", true)
  image_name            = local.instance_class["imageName"]
  tags                  = lookup(var.providerClusterConfiguration, "tags", {})
  ssh_allow_list        = lookup(var.providerClusterConfiguration, "sshAllowList", ["0.0.0.0/0"])
  server_group          = lookup(local.ng, "serverGroup", {})
  server_group_policy   = lookup(local.server_group, "policy", "")
  security_group_names  = local.network_security ? concat([local.prefix], lookup(local.instance_class, "additionalSecurityGroups", [])) : []
  volume_type_map       = lookup(local.ng, "volumeTypeMap", var.providerClusterConfiguration.masterNodeGroup.volumeTypeMap)
  actual_zones          = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.huaweicloud_availability_zones.zones.names, var.providerClusterConfiguration.zones)) : data.huaweicloud_availability_zones.zones.names
  zone                  = element(tolist(setintersection(keys(local.volume_type_map), local.actual_zones)), var.nodeIndex)
  volume_type           = local.volume_type_map[local.zone]
  flavor_name           = local.instance_class["flavorName"]
  root_disk_size        = lookup(local.instance_class, "rootDiskSize", 50) # Huaweicloud can have disks predefined within vm flavours, so we do not set any defaults here
  additional_tags       = lookup(local.instance_class, "additionalTags", {})
}

data "huaweicloud_vpc_subnet" "fallback" {
  name = var.providerClusterConfiguration.vpcPeering.subnet
}

locals {
  fallback_primary_subnet_id = data.huaweicloud_vpc_subnet.fallback.id

  main_network_id = coalesce(
    var.mainNetwork,
    lookup(local.instance_class, "mainNetwork", null),
    local.fallback_primary_subnet_id
  )

  additional_network_ids = (
    length(var.additionalNetworks) > 0
    ? var.additionalNetworks
    : (
        try(type(local.instance_class.additionalNetworks) == string, false)
        ? [local.instance_class.additionalNetworks]
        : try(local.instance_class.additionalNetworks, [])
      )
  )
  enterprise_project_id = lookup(var.providerClusterConfiguration.provider, "enterpriseProjectID", "")
}
