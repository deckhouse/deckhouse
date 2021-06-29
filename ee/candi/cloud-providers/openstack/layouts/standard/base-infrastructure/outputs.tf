# Copyright 2021 Flant CJSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/ee/LICENSE

output "cloud_discovery_data" {
  value = {
    "apiVersion" = "deckhouse.io/v1"
    "kind" = "OpenStackCloudDiscoveryData"
    "layout" = var.providerClusterConfiguration.layout
    "internalNetworkNames" = [openstack_networking_network_v2.internal.name]
    "externalNetworkNames" = [data.openstack_networking_network_v2.external.name]
    "podNetworkMode" = local.network_security ? "DirectRoutingWithPortSecurityEnabled" : "DirectRouting"
    "instances" = {
      "sshKeyPairName" = module.keypair.ssh_name
      "imageName" = local.image_name
      "mainNetwork" = openstack_networking_network_v2.internal.name
      "securityGroups" = module.network_security.security_group_names
    }
    "zones" = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.openstack_compute_availability_zones_v2.zones.names, var.providerClusterConfiguration.zones)) : data.openstack_compute_availability_zones_v2.zones.names
    "loadBalancer" = {
      "subnetID" = openstack_networking_subnet_v2.internal.id
      "floatingNetworkID" = data.openstack_networking_network_v2.external.id
    }
  }
}
