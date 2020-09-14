output "cloud_discovery_data" {
  value = {
    "apiVersion" = "deckhouse.io/v1alpha1"
    "kind" = "OpenStackCloudDiscoveryData"
    "internalNetworkNames" = [openstack_networking_network_v2.internal.name]
    "externalNetworkNames" = [data.openstack_networking_network_v2.external.name]
    "podNetworkMode" = local.network_security ? "DirectRoutingWithPortSecurityEnabled" : "DirectRouting"
    "instances" = {
      "sshKeyPairName" = module.keypair.ssh_name
      "securityGroups" = module.network_security.security_group_names
    }
    "zones" = data.openstack_compute_availability_zones_v2.zones.names
    "loadBalancer" = {
      "subnetID" = openstack_networking_subnet_v2.internal.id
      "floatingNetworkID" = data.openstack_networking_network_v2.external.id
    }
  }
}
