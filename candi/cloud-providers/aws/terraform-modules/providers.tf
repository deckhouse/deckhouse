provider "aws" {
  access_key = var.providerClusterConfiguration.provider.providerAccessKeyId
  secret_key = var.providerClusterConfiguration.provider.providerSecretAccessKey
  region = var.providerClusterConfiguration.provider.region
}
