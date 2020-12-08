provider "azurerm" {
  subscription_id = var.providerClusterConfiguration.provider.subscriptionId
  client_id       = var.providerClusterConfiguration.provider.clientId
  client_secret   = var.providerClusterConfiguration.provider.clientSecret
  tenant_id       = var.providerClusterConfiguration.provider.tenantId

  features {}
}
