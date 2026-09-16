# Copyright 2026 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "providerClusterConfiguration" {
  type    = any
  default = null
}

variable "nodeGroups" {
  type    = any
  default = {}
}

variable "instanceClasses" {
  type    = any
  default = {}
}

variable "secrets" {
  type    = any
  default = {}
}

variable "settings" {
  type    = any
  default = null
}
