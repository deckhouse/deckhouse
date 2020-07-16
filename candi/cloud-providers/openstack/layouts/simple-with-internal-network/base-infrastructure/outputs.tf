locals {
  data_1 = {
    "internalNetworkNames" = [data.openstack_networking_network_v2.internal.name]
    "podNetworkMode" = local.pod_network_mode
    "instances" = {
      "sshKeyPairName" = module.keypair.ssh_name
    }
    "zones" = data.openstack_compute_availability_zones_v2.zones.names
  }
  data_load_balancer = {
    "loadBalancer": {
      "subnetID" = data.openstack_networking_subnet_v2.internal.id
      "floatingNetworkID" = local.external_network_name == "" ? "" : data.openstack_networking_network_v2.external[0].id
    }
  }
  data_external_network_names = {
    "externalNetworkNames" = local.external_network_name == "" ? [] : [data.openstack_networking_network_v2.external[0].id]
  }
  data_2 = local.external_network_name == "" ? merge(local.data_1, {"loadBalancer": {}}) : merge(local.data_1, local.data_load_balancer)
  cloud_discovery_data = local.external_network_name == "" ? merge(local.data_2, {"externalNetworkNames": []}) : merge(local.data_2, local.data_external_network_names)
}

output "cloud_discovery_data" {
  value = local.cloud_discovery_data
}
