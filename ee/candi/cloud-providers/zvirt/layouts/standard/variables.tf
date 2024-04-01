# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

variable "clusterConfiguration" {
  type = any
}

variable "providerClusterConfiguration" {
  type = any
}

variable "url" {
  description = "oVirt API URL"
}

variable "username" {
  description = "zVirt Admin user"
}

variable "password" {
  description = "zVirt Admin password"
}

variable "insecure_mode" {
  description = "TLS validation"
}

