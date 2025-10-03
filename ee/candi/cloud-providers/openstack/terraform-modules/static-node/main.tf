# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  actual_zones = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.openstack_compute_availability_zones_v2.zones.names, var.providerClusterConfiguration.zones)) : data.openstack_compute_availability_zones_v2.zones.names
  zones        = lookup(local.ng, "zones", null) != null ? tolist(setintersection(local.actual_zones, local.ng["zones"])) : local.actual_zones
  volume_type_map      = lookup(local.ng, "volumeTypeMap", {})
  zone                 = local.volume_type_map != {} ? element(tolist(setintersection(keys(local.volume_type_map), local.zones)), var.nodeIndex) : element(tolist(local.zones), var.nodeIndex)
  volume_type          = local.volume_type_map != {} ? local.volume_type_map[local.zone] : null
}

module "security_groups" {
  source               = "../../../terraform-modules/security-groups"
  security_group_names = local.security_group_names
}

module "volume_zone" {
  source       = "../../../terraform-modules/volume-zone"
  compute_zone = element(local.zones, var.nodeIndex)
  region       = var.providerClusterConfiguration.provider.region
}

data "openstack_compute_availability_zones_v2" "zones" {}

data "openstack_images_image_v2" "image" {
  name = local.image_name
}

data "openstack_networking_network_v2" "network" {
  count = length(local.networks)
  name  = local.networks[count.index]
}

resource "openstack_networking_port_v2" "port" {
  count              = length(local.networks)
  name               = join("-", [local.prefix, var.nodeGroupName, var.nodeIndex])
  network_id         = data.openstack_networking_network_v2.network[count.index].id
  admin_state_up     = "true"
  security_group_ids = try(index(local.networks_with_security_disabled, data.openstack_networking_network_v2.network[count.index].name), -1) == -1 ? module.security_groups.security_group_ids : []

  dynamic "allowed_address_pairs" {
    for_each = local.internal_network_security_enabled && local.networks[count.index] == local.network_with_port_security ? list(local.pod_subnet_cidr) : []
    content {
      ip_address = allowed_address_pairs.value
    }
  }
}

resource "openstack_blockstorage_volume_v3" "volume" {
  count             = local.root_disk_size == "" ? 0 : 1
  name              = join("-", [local.prefix, var.nodeGroupName, var.nodeIndex])
  size              = local.root_disk_size
  image_id          = data.openstack_images_image_v2.image.id
  metadata          = local.metadata_tags
  volume_type       = local.volume_type
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

resource "openstack_compute_instance_v2" "node" {
  name              = join("-", [local.prefix, var.nodeGroupName, var.nodeIndex])
  image_name        = data.openstack_images_image_v2.image.name
  flavor_name       = local.flavor_name
  key_pair          = local.prefix
  config_drive      = local.config_drive
  user_data         = var.cloudConfig == "" ? "" : base64decode(var.cloudConfig)
  availability_zone = element(local.zones, var.nodeIndex)

  dynamic "network" {
    for_each = openstack_networking_port_v2.port

    content {
      port = network.value["id"]
    }
  }

  dynamic "block_device" {
    for_each = local.root_disk_size == "" ? [] : list(openstack_blockstorage_volume_v3.volume[0])
    content {
      uuid                  = block_device.value["id"]
      boot_index            = 0
      source_type           = "volume"
      destination_type      = "volume"
      delete_on_termination = true
    }
  }

  lifecycle {
    ignore_changes = [
      user_data,
    ]
  }

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
    update = var.resourceManagementTimeout
  }

  metadata = length(local.metadata_tags) > 0 ? local.metadata_tags : {}
}

resource "openstack_compute_floatingip_v2" "floating_ip" {
  count = length(local.floating_ip_pools)
  pool  = local.floating_ip_pools[count.index]
}

resource "openstack_compute_floatingip_associate_v2" "node" {
  count                 = length(local.floating_ip_pools)
  floating_ip           = openstack_compute_floatingip_v2.floating_ip[count.index].address
  instance_id           = openstack_compute_instance_v2.node.id
  wait_until_associated = true

  lifecycle {
    ignore_changes = [
      wait_until_associated,
    ]
  }
}
