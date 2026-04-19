# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "organization" {
  type = string
}

variable "vdc_name" {
  type = string
}

variable "prefix" {
  type = string
}

variable "vapp_name" {
  type = string
}

variable "network_name" {
  type = string
}

variable "ip_address" {
  type    = string
  default = ""
}

variable "template" {
  type = string
}

variable "ssh_public_key" {
  type    = string
  default = ""
}

variable "placement_policy" {
  type    = string
  default = ""
}

variable "storage_profile" {
  type = string
}

variable "sizing_policy" {
  type = string
}

variable "root_disk_size_gb" {
  type = number
}

variable "metadata" {
  type    = map(string)
  default = {}
}

locals {
  name           = format("%s-bastion", var.prefix)
  template_parts = split("/", var.template)
  org            = length(local.template_parts) == 3 ? local.template_parts[0] : null
  catalog        = length(local.template_parts) == 3 ? local.template_parts[1] : local.template_parts[0]
  template       = length(local.template_parts) == 3 ? local.template_parts[2] : local.template_parts[1]
}
