output "cloud_discovery_data" {
  value = {
    "apiVersion"        = "deckhouse.io/v1alpha1"
    "kind"              = "AzureCloudDiscoveryData"
    "resourceGroupName" = azurerm_resource_group.kube.name
    "vnetName"          = azurerm_virtual_network.kube.name
    "subnetName"        = azurerm_subnet.kube.name
    "zones"             = local.zones
    "instances" = {
      "urn"            = var.providerClusterConfiguration.masterNodeGroup.instanceClass.urn
      "diskType"       = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "diskType", "StandardSSD_LRS")
      "additionalTags" = lookup(var.providerClusterConfiguration, "tags", {})
    }
  }
}
