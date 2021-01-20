output "cloud_discovery_data" {
  value = {
    "apiVersion" = "deckhouse.io/v1alpha1"
    "kind" = "VsphereCloudDiscoveryData"
    "vmFolderPath" = vsphere_folder.main.path
    "resourcePoolPath" = local.base_resource_pool != "" ? join("/", [local.base_resource_pool, local.prefix]) : local.prefix
  }
}
