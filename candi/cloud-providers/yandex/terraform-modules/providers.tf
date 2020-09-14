provider "yandex" {
  cloud_id = var.providerClusterConfiguration.provider.cloudID
  folder_id = var.providerClusterConfiguration.provider.folderID
  service_account_key_file = var.providerClusterConfiguration.provider.serviceAccountJSON
}
