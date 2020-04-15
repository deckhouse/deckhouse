provider "openstack" {
  auth_url = var.clusterProviderConfig.provider.authURL
  default_domain = var.clusterProviderConfig.provider.domainName
  cacert_file = lookup(var.clusterProviderConfig.provider, "caCert", "")
  tenant_name = lookup(var.clusterProviderConfig.provider, "tenantName", "")
  tenant_id = lookup(var.clusterProviderConfig.provider, "tenantID", "")
  user_name = var.clusterProviderConfig.provider.username
  password = var.clusterProviderConfig.provider.password
  region = var.clusterProviderConfig.provider.region
}
