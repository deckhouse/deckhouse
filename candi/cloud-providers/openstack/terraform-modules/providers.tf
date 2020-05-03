provider "openstack" {
  auth_url = var.providerClusterConfig.provider.authURL
  domain_name = var.providerClusterConfig.provider.domainName
  cacert_file = lookup(var.providerClusterConfig.provider, "caCert", "")
  tenant_name = lookup(var.providerClusterConfig.provider, "tenantName", "")
  tenant_id = lookup(var.providerClusterConfig.provider, "tenantID", "")
  user_name = var.providerClusterConfig.provider.username
  password = var.providerClusterConfig.provider.password
  region = var.providerClusterConfig.provider.region
}
