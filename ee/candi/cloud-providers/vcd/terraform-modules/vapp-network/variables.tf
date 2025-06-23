# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "providerClusterConfiguration" {
  type = any

  validation {
    condition     = cidrsubnet(var.providerClusterConfiguration.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.internalNetworkCIDR
    error_message = "Invalid internalNetworkCIDR in VCDClusterConfiguration."
  }

  validation {
    condition     = length(flatten([for ng in var.providerClusterConfiguration.nodeGroups : ng.instanceClass.mainNetworkIPAddresses if contains(keys(ng.instanceClass), "mainNetworkIPAddresses")])) == length(flatten([for ng in var.providerClusterConfiguration.nodeGroups : [for s in ng.instanceClass.mainNetworkIPAddresses : s if cidrsubnet(format("%s/%s", s, split("/", var.providerClusterConfiguration.internalNetworkCIDR)[1]), 0, 0) == var.providerClusterConfiguration.internalNetworkCIDR] if contains(keys(ng.instanceClass), "mainNetworkIPAddresses")]))
    error_message = "Address in mainNetworkIPAddresses not in internalNetworkCIDR."
  }
}
