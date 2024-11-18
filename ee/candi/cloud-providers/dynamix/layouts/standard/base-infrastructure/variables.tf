# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

locals {
  resource_name_prefix = var.clusterConfiguration.cloud.prefix
  account = lookup(var.providerClusterConfiguration.provider, "account", null)
  location = lookup(var.providerClusterConfiguration, "location", null)
  resource_group_name = join("-", [local.resource_name_prefix, "rg"])
}

