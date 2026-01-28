# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "organization" {
  type = string
}

variable "vapp_name" {
  type = string
}

variable "metadata" {
  type    = map(string)
  default = {}
}
