

provider "kubernetes" {
  config_data_base64 = var.providerClusterConfiguration.provider.kubeconfigDataBase64
}
