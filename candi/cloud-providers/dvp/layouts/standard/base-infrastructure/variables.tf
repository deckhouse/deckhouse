variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "nodeIndex" {
  type    = string
  default = 0
}

variable "cloudConfig" {
  type = string
}

variable "clusterUUID" {
  type = string
}
