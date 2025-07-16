# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "organization" {
  type = string
}

variable "edge_gateway_name" {
  type = string
}

variable "edge_gateway_type" {
  type = string
}

variable "internal_network_name" {
  type = string
}

variable "internal_network_cidr" {
  type = string

  validation {
    condition     = contains(keys(var.providerClusterConfiguration), "internalNetworkCIDR") ? cidrsubnet(var.providerClusterConfiguration.internalNetworkCIDR, 0, 0) == var.providerClusterConfiguration.internalNetworkCIDR : true
    error_message = format("%s is not valid CIDR.", var.internal_network_cidr)
  }
}

variable "external_network_name" {
  type = string
}

variable "external_network_type" {
  type = string
}

variable "external_address" {
  type = string
}

variable "external_port" {
  type = number
}
