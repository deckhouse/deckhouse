# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

provider "vcd" {
  url = join("/", [trimsuffix(var.providerClusterConfiguration.provider.server, "/api"), "api"])
  org = var.providerClusterConfiguration.organization
  vdc = var.providerClusterConfiguration.virtualDataCenter
  user = var.providerClusterConfiguration.provider.username
  password = var.providerClusterConfiguration.provider.password
  auth_type = "integrated"
  allow_unverified_ssl = lookup(var.providerClusterConfiguration.provider, "insecure", false)
  max_retry_timeout = 60
}
