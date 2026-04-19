# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "prefix" {
  type = string
}

variable "enabled" {
  type = bool
  default = true
}

variable "ssh_allow_list" {
  type = list(string)
}

variable "enterprise_project_id" {
  type = string
}
