# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "prefix" {
  type = string
}

variable "node_index" {
  type = string
}

variable "master_id" {
  type = string
}

variable "volume_size" {
  type = number
}

variable "volume_type" {
  type = string
}

variable "volume_zone" {
  type = string
}

variable "tags" {
  type = map(string)
}

variable "enterprise_project_id" {
  type = string
}
