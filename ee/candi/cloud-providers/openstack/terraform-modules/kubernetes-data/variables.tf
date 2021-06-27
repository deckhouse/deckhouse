# Copyright 2021 Flant CJSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE

variable "prefix" {
  type = string
}

variable "node_index" {
  type = string
}

variable "master_id" {
  type = string
}

variable "volume_type" {
  type = string
}

variable "tags" {
  type = map(string)
}
