# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

provider "decort" {
  authenticator        = "decs3o"
  controller_url       = var.providerClusterConfiguration.provider.controllerUrl
  oauth2_url           = var.providerClusterConfiguration.provider.oAuth2Url
  allow_unverified_ssl = lookup(var.providerClusterConfiguration.provider, "insecure", false)
  app_id               = var.providerClusterConfiguration.provider.appId
  app_secret           = var.providerClusterConfiguration.provider.appSecret
}
