# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

provider "openstack" {
  auth_url                      = var.providerClusterConfiguration.provider.authURL
  domain_name                   = lookup(var.providerClusterConfiguration.provider, "domainName", "")
  cacert_file                   = lookup(var.providerClusterConfiguration.provider, "caCert", "")
  tenant_name                   = lookup(var.providerClusterConfiguration.provider, "tenantName", "")
  tenant_id                     = lookup(var.providerClusterConfiguration.provider, "tenantID", "")
  user_name                     = lookup(var.providerClusterConfiguration.provider, "username", "")
  password                      = lookup(var.providerClusterConfiguration.provider, "password", "")
  region                        = var.providerClusterConfiguration.provider.region
  application_credential_id     = lookup(var.providerClusterConfiguration.provider, "applicationCredentialId", "")
  application_credential_name   = lookup(var.providerClusterConfiguration.provider, "applicationCredentialName", "")
  application_credential_secret = lookup(var.providerClusterConfiguration.provider, "applicationCredentialSecret", "")
  token                         = lookup(var.providerClusterConfiguration.provider, "token", "")
}
