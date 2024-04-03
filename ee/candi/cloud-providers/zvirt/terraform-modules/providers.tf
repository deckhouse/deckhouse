# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

provider "ovirt" {
  url = var.providerClusterConfiguration.provider.server
  username = var.providerClusterConfiguration.provider.username
  password = var.providerClusterConfiguration.provider.password
  tls_insecure = lookup(var.providerClusterConfiguration.provider, "insecure", false)
  tls_ca_bundle = base64decode(lookup(var.providerClusterConfiguration.provider, "caBundle", ""))
}
