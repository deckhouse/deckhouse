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
    condition     = cidrsubnet(var.internal_network_cidr, 0, 0) == var.internal_network_cidr
    error_message = "Content of the internal_network_cidr is not valid network CIDR."
  }
}
