# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

provider "openstack" {
  auth_url    = var.providerClusterConfiguration.provider.authURL
  domain_name = var.providerClusterConfiguration.provider.domainName
  cacert_file = lookup(var.providerClusterConfiguration.provider, "caCert", "")
  tenant_name = lookup(var.providerClusterConfiguration.provider, "tenantName", "")
  tenant_id   = lookup(var.providerClusterConfiguration.provider, "tenantID", "")
  user_name   = var.providerClusterConfiguration.provider.username
  password    = var.providerClusterConfiguration.provider.password
  region      = var.providerClusterConfiguration.provider.region
}
