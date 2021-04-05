variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
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
  associate_public_ip_to_masters = lookup(var.providerClusterConfiguration.standard, "associatePublicIPToMasters", false)
  root_volume_size = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskSizeGb", 20)
  root_volume_type = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskType", "gp2")
  additional_security_groups = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalSecurityGroups", [])
  actual_zones = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.aws_availability_zones.available.names, var.providerClusterConfiguration.zones)) : data.aws_availability_zones.available.names
  zones = lookup(var.providerClusterConfiguration.masterNodeGroup, "zones", null) != null ? tolist(setintersection(local.actual_zones, var.providerClusterConfiguration.masterNodeGroup["zones"])) : local.actual_zones
  tags = merge(lookup(var.providerClusterConfiguration, "tags", {}), lookup(var.providerClusterConfiguration.masterNodeGroup, "additionalTags", {}))
}
