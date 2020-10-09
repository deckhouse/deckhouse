module "security_groups" {
  source = "/deckhouse/candi/cloud-providers/openstack/terraform-modules/security-groups"
  security_group_names = local.security_group_names
}

data "openstack_images_image_v2" "image" {
  name = local.image_name
}

data "openstack_networking_network_v2" "network" {
  count = length(local.networks)
  name = local.networks[count.index]
}

resource "openstack_networking_port_v2" "port" {
  count = length(local.networks)
  name = join("-", [local.prefix, var.nodeGroupName, var.nodeIndex])
  network_id = data.openstack_networking_network_v2.network[count.index].id
  admin_state_up = "true"
  security_group_ids = try(index(local.networks_with_security_disabled, data.openstack_networking_network_v2.network[count.index].name), -1) == -1 ? module.security_groups.security_group_ids : []

  dynamic "allowed_address_pairs" {
    for_each = local.internal_network_security_enabled && local.networks[count.index] == local.prefix ? list(local.pod_subnet_cidr) : []

    content {
      ip_address = allowed_address_pairs.value
    }
  }
}

resource "openstack_blockstorage_volume_v2" "volume" {
  count = local.root_disk_size == "" ? 0 : 1
  name = join("-", [local.prefix, var.nodeGroupName, var.nodeIndex])
  size = local.root_disk_size
  image_id = data.openstack_images_image_v2.image.id
  metadata = local.metadata_tags
}

resource "openstack_compute_instance_v2" "node" {
  name = join("-", [local.prefix, var.nodeGroupName, var.nodeIndex])
  image_name = data.openstack_images_image_v2.image.name
  flavor_name = local.flavor_name
  key_pair = local.prefix
  config_drive = local.config_drive
  user_data = var.cloudConfig == "" ? "" : base64decode(var.cloudConfig)
  availability_zone = local.zones == null ? null : element(local.zones, var.nodeIndex)

  dynamic "network" {
    for_each = openstack_networking_port_v2.port

    content {
      port = network.value["id"]
    }
  }

  dynamic "block_device" {
    for_each = local.root_disk_size == "" ? [] : list(openstack_blockstorage_volume_v2.volume[0])
    content {
      uuid = block_device.value["id"]
      boot_index = 0
      source_type = "volume"
      destination_type = "volume"
      delete_on_termination = true
    }
  }

  lifecycle {
    ignore_changes = [
      user_data,
    ]
  }

  metadata = local.metadata_tags
}

resource "openstack_compute_floatingip_v2" "floating_ip" {
  count = length(local.floating_ip_pools)
  pool = local.floating_ip_pools[count.index]
}

resource "openstack_compute_floatingip_associate_v2" "node" {
  count = length(local.floating_ip_pools)
  floating_ip = openstack_compute_floatingip_v2.floating_ip[count.index].address
  instance_id = openstack_compute_instance_v2.node.id
}

