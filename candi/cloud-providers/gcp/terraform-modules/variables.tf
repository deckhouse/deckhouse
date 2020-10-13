variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "nodeIndex" {
  type = string
}

variable "cloudConfig" {
  type = string
  default = ""
}

variable "clusterUUID" {
  type = string
}
