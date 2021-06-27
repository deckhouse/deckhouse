# Copyright 2021 Flant CJSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE

output "cloud_discovery_data" {
  value = {
    "apiVersion" = "deckhouse.io/v1alpha1"
    "kind" = "OpenStackCloudDiscoveryData"
    "layout" = var.providerClusterConfiguration.layout
    "internalNetworkNames" = [data.openstack_networking_network_v2.external.name]
    "podNetworkMode" = local.pod_network_mode
    "instances" = {
      "sshKeyPairName" = module.keypair.ssh_name
      "imageName" = local.image_name
      "mainNetwork" = data.openstack_networking_network_v2.external.name
    }
    "zones" = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.openstack_compute_availability_zones_v2.zones.names, var.providerClusterConfiguration.zones)) : data.openstack_compute_availability_zones_v2.zones.names
  }
}
