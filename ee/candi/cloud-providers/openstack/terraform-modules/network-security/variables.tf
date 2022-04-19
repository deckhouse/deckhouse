# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

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

variable "ssh_allow_list" {
  type = list(string)
}
