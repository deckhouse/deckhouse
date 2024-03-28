# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

provider "ovirt" {
  url = var.providerClusterConfiguration.provider.server
  username = var.providerClusterConfiguration.provider.username
  password = var.providerClusterConfiguration.provider.password
  tls_insecure = var.providerClusterConfiguration.provider.insecure
}
