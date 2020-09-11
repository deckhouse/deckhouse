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

locals {
  prefix = var.clusterConfiguration.cloud.prefix
  root_volume_size = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskSizeGb", 20)
  root_volume_type = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskType", "gp2")
  additional_security_groups = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalSecurityGroups", [])
}
