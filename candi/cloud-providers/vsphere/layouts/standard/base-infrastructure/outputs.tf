output "cloud_discovery_data" {
  value = {
    "apiVersion" = "deckhouse.io/v1alpha1"
    "kind" = "VsphereCloudDiscoveryData"
    "vmFolderPath" = vsphere_folder.main.path
  }
}
