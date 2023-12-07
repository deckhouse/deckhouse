# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

provider "vcd" {
  url = endswith(var.providerClusterConfiguration.provider.server, "/api") ? var.providerClusterConfiguration.provider.server : join("/", [var.providerClusterConfiguration.provider.server, "api"])
  org = var.providerClusterConfiguration.organization
  vdc = var.providerClusterConfiguration.virtualDataCenter
  user = var.providerClusterConfiguration.provider.username
  password = var.providerClusterConfiguration.provider.password
  auth_type = "integrated"
  allow_unverified_ssl = lookup(var.providerClusterConfiguration.provider, "insecure", false)
  max_retry_timeout = 60
}
