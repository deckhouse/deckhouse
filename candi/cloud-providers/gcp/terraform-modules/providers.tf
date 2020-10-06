provider "google" {
  credentials = var.providerClusterConfiguration.provider.serviceAccountJSON
  project     = jsondecode(var.providerClusterConfiguration.provider.serviceAccountJSON).project_id
  region      = var.providerClusterConfiguration.provider.region
  # Should be specified in region, probably we can skip it here
  zone = "${var.providerClusterConfiguration.provider.region}-a"
}
