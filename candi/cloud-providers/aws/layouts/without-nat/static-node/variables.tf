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
  type = any
  default = ""
}

variable "clusterUUID" {
  type = string
}

data "aws_availability_zones" "available" {}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  node_groups = lookup(var.providerClusterConfiguration, "nodeGroups", [])
  node_group = [for i in local.node_groups: i if i.name == var.nodeGroupName][0]
  root_volume_size = lookup(local.node_group.instanceClass, "diskSizeGb", 20)
  root_volume_type = lookup(local.node_group.instanceClass, "diskType", "gp2")
  additional_security_groups = lookup(local.node_group.instanceClass, "additionalSecurityGroups", [])
  actual_zones = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.aws_availability_zones.available.names, var.providerClusterConfiguration.zones)) : data.aws_availability_zones.available.names
  zones = lookup(local.node_group, "zones", null) != null ? tolist(setintersection(local.actual_zones, local.node_group["zones"])) : local.actual_zones
  tags = merge(lookup(var.providerClusterConfiguration, "tags", {}), lookup(local.node_group, "additionalTags", {}))
}
