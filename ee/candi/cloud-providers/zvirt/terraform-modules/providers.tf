# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

provider "ovirt" {
  url = var.url
  username = var.username
  password = var.password
  tls_insecure = var.insecure
}
