# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

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

variable "clusterUUID" {
  type = string
}

locals {
  prefix = var.clusterConfiguration.cloud.prefix
}
