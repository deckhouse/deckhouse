# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  network_security = local.pod_network_mode == "DirectRoutingWithPortSecurityEnabled"
  security_group_names = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalSecurityGroups", [])
  external_network_floating_ip = lookup(var.providerClusterConfiguration.simpleWithInternalNetwork, "masterWithExternalFloatingIP", true)
  volume_type_map = var.providerClusterConfiguration.masterNodeGroup.volumeTypeMap
  actual_zones = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.openstack_compute_availability_zones_v2.zones.names, var.providerClusterConfiguration.zones)) : data.openstack_compute_availability_zones_v2.zones.names
  zone = element(tolist(setintersection(keys(local.volume_type_map), local.actual_zones)), var.nodeIndex)
  volume_type = local.volume_type_map[local.zone]
  flavor_name = var.providerClusterConfiguration.masterNodeGroup.instanceClass.flavorName
  root_disk_size = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "rootDiskSize", "")
  etcd_volume_size = var.providerClusterConfiguration.masterNodeGroup.instanceClass.etcdDiskSizeGb
  additional_tags = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalTags", {})
  server_group = lookup(var.providerClusterConfiguration.masterNodeGroup, "serverGroup", {})
}

module "volume_zone" {
  source = "../../../terraform-modules/volume-zone"
  compute_zone = local.zone
  region = var.providerClusterConfiguration.provider.region
}

module "master" {
  source = "../../../terraform-modules/master"
  prefix = local.prefix
  node_index = var.nodeIndex
  cloud_config = var.cloudConfig
  flavor_name = local.flavor_name
  root_disk_size = local.root_disk_size
  additional_tags = local.additional_tags
  image_name = local.image_name
  keypair_ssh_name = data.openstack_compute_keypair_v2.ssh.name
  network_port_ids = list(local.network_security ? openstack_networking_port_v2.master_internal_with_security[0].id : openstack_networking_port_v2.master_internal_without_security[0].id)
  internal_network_cidr = data.openstack_networking_subnet_v2.internal.cidr
  floating_ip_network = local.external_network_floating_ip ? local.external_network_name : ""
  tags = local.tags
  zone = local.zone
  volume_type = local.volume_type
  volume_zone = module.volume_zone.zone
  server_group = local.server_group
}

module "kubernetes_data" {
  source = "../../../terraform-modules/kubernetes-data"
  prefix = local.prefix
  node_index = var.nodeIndex
  master_id = module.master.id
  volume_size = local.etcd_volume_size
  volume_type = local.volume_type
  volume_zone = module.volume_zone.zone
  tags = local.tags
}

module "security_groups" {
  source = "../../../terraform-modules/security-groups"
  security_group_names = local.security_group_names
}

data "openstack_compute_availability_zones_v2" "zones" {}

data "openstack_compute_keypair_v2" "ssh" {
  name = local.prefix
}

data "openstack_networking_subnet_v2" "internal" {
  name = local.internal_subnet_name
}

data "openstack_networking_network_v2" "internal" {
  network_id = data.openstack_networking_subnet_v2.internal.network_id
}

resource "openstack_networking_port_v2" "master_internal_with_security" {
  count = local.network_security ? 1 : 0
  network_id = data.openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  security_group_ids = module.security_groups.security_group_ids
  fixed_ip {
    subnet_id = data.openstack_networking_subnet_v2.internal.id
  }
  allowed_address_pairs {
    ip_address = local.pod_subnet_cidr
  }
}

resource "openstack_networking_port_v2" "master_internal_without_security" {
  count = local.network_security ? 0 : 1
  network_id = data.openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  security_group_ids = module.security_groups.security_group_ids
  fixed_ip {
    subnet_id = data.openstack_networking_subnet_v2.internal.id
  }
}
