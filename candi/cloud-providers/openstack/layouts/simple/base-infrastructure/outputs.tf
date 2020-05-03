output "deckhouse_config" {
  value = {
    "nginxIngressEnabled": false
    "prometheusMadisonIntegrationEnabled": false
  }
}

output "cloud_discovery_data" {
  value = {
    "internalNetworkNames" = [data.openstack_networking_network_v2.external.name]
    "podNetworkMode" = local.pod_network_mode
    "instances" = {
      "sshKeyPairName" = module.keypair.ssh_name
    }
    "zones" = data.openstack_compute_availability_zones_v2.zones.names
  }
}
