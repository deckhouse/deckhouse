output "cloud_discovery_data" {
  value = {
    "apiVersion"        = "deckhouse.io/v1alpha1"
    "kind"              = "GCPCloudDiscoveryData"
    "networkName"       = google_compute_network.kube.name
    "subnetworkName"    = google_compute_subnetwork.kube.name
    "zones"             = data.google_compute_zones.available.names
    "disableExternalIP" = var.providerClusterConfiguration.layout == "WithoutNAT" ? false : true
    "instances" = {
      "image"       = var.providerClusterConfiguration.masterNodeGroup.instanceClass.image
      "diskSizeGb"  = 50
      "diskType"    = "pd-standard"
      "networkTags" = [local.prefix]
      "labels"      = lookup(var.providerClusterConfiguration, "labels", {})
    }
  }
}
