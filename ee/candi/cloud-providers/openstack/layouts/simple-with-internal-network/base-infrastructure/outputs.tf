# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  data_1 = {
    "layout" = var.providerClusterConfiguration.layout
    "internalNetworkNames" = [data.openstack_networking_network_v2.internal.name]
    "podNetworkMode" = local.pod_network_mode
    "instances" = {
      "sshKeyPairName" = module.keypair.ssh_name
      "imageName" = local.image_name
      "mainNetwork" = data.openstack_networking_network_v2.internal.name
    }
    "zones" = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.openstack_compute_availability_zones_v2.zones.names, var.providerClusterConfiguration.zones)) : data.openstack_compute_availability_zones_v2.zones.names
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
  value = merge(local.cloud_discovery_data, {"apiVersion": "deckhouse.io/v1","kind":"OpenStackCloudDiscoveryData"})
}
