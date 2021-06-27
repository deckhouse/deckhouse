# Copyright 2021 Flant CJSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE

variable "prefix" {
  type = string
}

variable "remote_ip_prefix" {
  type = string
}

variable "enabled" {
  type = bool
  default = true
}
