# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "layout_security_group_ids" {
  default = []
  type = list(string)
}

variable "layout_security_group_names" {
  default = []
  type = list(string)
}

variable "security_group_names" {
  type = list(string)
}

variable "enterprise_project_id" {
  type = string
}
