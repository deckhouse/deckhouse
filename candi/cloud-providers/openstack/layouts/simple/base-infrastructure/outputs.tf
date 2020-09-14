output "cloud_discovery_data" {
  value = {
    "apiVersion" = "deckhouse.io/v1alpha1"
    "kind" = "OpenStackCloudDiscoveryData"
    "internalNetworkNames" = [data.openstack_networking_network_v2.external.name]
    "podNetworkMode" = local.pod_network_mode
    "instances" = {
      "sshKeyPairName" = module.keypair.ssh_name
    }
    "zones" = data.openstack_compute_availability_zones_v2.zones.names
  }
}
