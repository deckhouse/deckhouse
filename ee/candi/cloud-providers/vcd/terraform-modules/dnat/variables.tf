# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "organization" {
  type = string
}

variable "rule_name_prefix" {
  type        = string
  description = "The prefix to the name of the DNAT rule. Effective only for NSX-T."
}

variable "rule_description" {
  type    = string
  default = ""
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

variable "internal_address" {
  type = string
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
