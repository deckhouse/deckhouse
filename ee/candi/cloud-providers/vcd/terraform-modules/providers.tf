# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

provider "vcd" {
  url                  = join("/", [trimsuffix(trimsuffix(var.providerClusterConfiguration.provider.server, "/"), "/api"), "api"])
  org                  = var.providerClusterConfiguration.organization
  vdc                  = var.providerClusterConfiguration.virtualDataCenter
  user                 = lookup(var.providerClusterConfiguration.provider, "username", "none")
  password             = lookup(var.providerClusterConfiguration.provider, "password", "none")
  api_token            = lookup(var.providerClusterConfiguration.provider, "apiToken", "")
  auth_type            = lookup(var.providerClusterConfiguration.provider, "apiToken", null) != null ? "api_token" : "integrated"
  allow_unverified_ssl = lookup(var.providerClusterConfiguration.provider, "insecure", false)
  max_retry_timeout    = 60
}
