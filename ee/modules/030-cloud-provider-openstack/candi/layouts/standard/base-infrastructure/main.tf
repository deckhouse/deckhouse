# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

module "network_security" {
  source           = "../../../terraform-modules/network-security"
  prefix           = local.prefix
  ssh_allow_list   = local.ssh_allow_list
  enabled          = local.network_security
}

module "keypair" {
  source         = "../../../terraform-modules/keypair"
  prefix         = local.prefix
  ssh_public_key = var.providerClusterConfiguration.sshPublicKey
}

data "openstack_compute_availability_zones_v2" "zones" {}

data "openstack_networking_network_v2" "external" {
  name = local.external_network_name
}

resource "openstack_networking_network_v2" "internal" {
  name           = local.prefix
  admin_state_up = "true"
}

resource "openstack_networking_subnet_v2" "internal" {
  name        = local.prefix
  network_id  = openstack_networking_network_v2.internal.id
  cidr        = local.internal_network_cidr
  ip_version  = 4
  gateway_ip  = cidrhost(local.internal_network_cidr, 1)
  enable_dhcp = "true"
  allocation_pool {
    start = cidrhost(local.internal_network_cidr, 2)
    end   = cidrhost(local.internal_network_cidr, 254)
  }
  dns_nameservers = var.providerClusterConfiguration.standard.internalNetworkDNSServers
}

resource "openstack_networking_router_v2" "router" {
  name                = local.prefix
  admin_state_up      = "true"
  external_network_id = data.openstack_networking_network_v2.external.id
}

resource "openstack_networking_router_interface_v2" "router" {
  router_id = openstack_networking_router_v2.router.id
  subnet_id = openstack_networking_subnet_v2.internal.id
}

// bastion and his friends

locals {
  bastion_instance   = lookup(local.standard, "bastion", {})
  instance_class     = lookup(local.bastion_instance, "instanceClass", {})
  name               = join("-", [local.prefix, "bastion"])
  bastion_image_name = lookup(local.instance_class, "imageName", null) != null ? local.instance_class.imageName : var.providerClusterConfiguration.masterNodeGroup.instanceClass.imageName
  flavor_name        = lookup(local.instance_class, "flavorName", null)
  actual_zones       = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.openstack_compute_availability_zones_v2.zones.names, var.providerClusterConfiguration.zones)) : data.openstack_compute_availability_zones_v2.zones.names
  zone               = lookup(local.bastion_instance, "zone", null) != null ? local.bastion_instance.zone : local.actual_zones[0]
  metadata_tags      = merge(lookup(var.providerClusterConfiguration, "tags", {}), lookup(local.instance_class, "additionalTags", {}))
  config_drive       = false
  root_disk_size     = lookup(local.instance_class, "rootDiskSize", "30")
  volume_type        = lookup(local.bastion_instance, "volumeType", null)
}

resource "openstack_networking_port_v2" "bastion" {
  count          = local.bastion_instance != {} ? 1 : 0
  name           = local.name
  network_id     = openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.internal.id
  }
}

data "openstack_images_image_v2" "image" {
  count = local.bastion_instance != {} ? 1 : 0
  name  = local.bastion_image_name
}

module "volume_zone" {
  source = "../../../terraform-modules/volume-zone"
  compute_zone = local.zone
  region = var.providerClusterConfiguration.provider.region
}

resource "openstack_blockstorage_volume_v3" "root" {
  count       = local.bastion_instance != {} ? 1 : 0
  name        = join("-", [local.name, "root-volume"])
  size        = local.root_disk_size
  image_id    = data.openstack_images_image_v2.image[0].id
  metadata    = local.metadata_tags
  volume_type = local.volume_type
  availability_zone = module.volume_zone.zone
  enable_online_resize = true

  lifecycle {
    ignore_changes = [
      metadata,
      availability_zone,
    ]
  }

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
  }
}

resource "openstack_compute_instance_v2" "bastion" {
  count             = local.bastion_instance != {} ? 1 : 0
  name              = local.name
  image_name        = local.bastion_image_name
  flavor_name       = local.flavor_name
  key_pair          = local.prefix
  config_drive      = local.config_drive
  availability_zone = local.zone

  network {
    port = openstack_networking_port_v2.bastion[0].id
  }

  block_device {
    uuid                  = openstack_blockstorage_volume_v3.root[0].id
    boot_index            = 0
    source_type           = "volume"
    destination_type      = "volume"
    delete_on_termination = true
  }

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
    update = var.resourceManagementTimeout
  }

  metadata = length(local.metadata_tags) > 0 ? local.metadata_tags : null
}

resource "openstack_compute_floatingip_v2" "bastion" {
  count = local.bastion_instance != {} ? 1 : 0
  pool  = data.openstack_networking_network_v2.external.name
}

resource "openstack_compute_floatingip_associate_v2" "bastion" {
  count                 = local.bastion_instance != {} ? 1 : 0
  floating_ip           = openstack_compute_floatingip_v2.bastion[0].address
  instance_id           = openstack_compute_instance_v2.bastion[0].id
  wait_until_associated = true
  lifecycle {
    ignore_changes = [
      wait_until_associated,
    ]
  }
}

resource "openstack_compute_servergroup_v2" "server_group" {
  count    = local.server_group_policy == "AntiAffinity" ? 1 : 0
  name     = local.prefix
  policies = ["anti-affinity"]
}
