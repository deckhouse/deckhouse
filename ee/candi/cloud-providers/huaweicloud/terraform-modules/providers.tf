# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

provider "huaweicloud" {
  cloud       = var.providerClusterConfiguration.provider.cloud
  region      = var.providerClusterConfiguration.provider.region
  access_key  = var.providerClusterConfiguration.provider.accessKey
  secret_key  = var.providerClusterConfiguration.provider.secretKey
  project_id  = lookup(var.providerClusterConfiguration.provider, "projectID", "")
  insecure    = lookup(var.providerClusterConfiguration.provider, "insecure", false)
  auth_url    = lookup(var.providerClusterConfiguration.provider, "authURL", "")
  domain_name = lookup(var.providerClusterConfiguration.provider, "domainName", "")
}
