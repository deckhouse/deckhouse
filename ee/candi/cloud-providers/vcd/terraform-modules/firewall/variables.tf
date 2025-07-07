# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "providerClusterConfiguration" {
  type = any

  validation {
    condition     = contains(keys(var.providerClusterConfiguration), "internalNetworkCIDR") ? cidrsubnet(var.providerClusterConfiguration.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.internalNetworkCIDR : true
    error_message = "Invalid internalNetworkCIDR in VCDClusterConfiguration."
  }
}

variable "edgeGatewayId" {
  type = string
}

variable "useNSXV" {
  type = bool
}
