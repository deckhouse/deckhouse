# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "prefix" {
  type = string
}

variable "image_name" {
  type = string
}

variable "flavor_name" {
  type = string
}

variable "root_disk_size" {
  type = number
}

variable "tags" {
  type = map(string)
}

variable "additional_tags" {
  type = map(string)
}

variable "keypair_ssh_name" {
  type = string
}

variable "enable_eip" {
  type    = bool
}

variable "security_group_ids" {
  type = list(string)
}

variable "internal_network_cidr" {
  type    = string
}

variable "node_index" {
  type = string
}

variable "cloud_config" {
  type = string
}

variable "volume_type" {
  type = string
}

variable "volume_zone" {
  type = string
}

variable "zone" {
  type = string
}

variable "server_group" {
  type = any
}

variable "subnet" {
  type = string
}

variable "enterprise_project_id" {
  type = string
}
