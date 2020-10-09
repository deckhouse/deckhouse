locals {
  metadata_tags = merge(var.tags, var.additional_tags)
}

data "openstack_images_image_v2" "master" {
  name = var.image_name
}

resource "openstack_blockstorage_volume_v2" "master" {
  count = var.root_disk_size == "" ? 0 : 1
  name = join("-", [var.prefix, "master-root-volume", var.node_index])
  size = var.root_disk_size
  image_id = data.openstack_images_image_v2.master.id
  metadata = local.metadata_tags
  volume_type = var.volume_type
}

resource "openstack_compute_instance_v2" "master" {
  name = join("-", [var.prefix, "master", var.node_index])
  image_name = data.openstack_images_image_v2.master.name
  flavor_name = var.flavor_name
  key_pair = var.keypair_ssh_name
  config_drive = var.config_drive
  user_data = var.cloud_config == "" ? null : base64decode(var.cloud_config)
  availability_zone = var.zone

  dynamic "network" {
    for_each = var.network_port_ids

    content {
      port = network.value
    }
  }

  dynamic "block_device" {
    for_each = var.root_disk_size == "" ? [] : list(openstack_blockstorage_volume_v2.master[0])
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

resource "openstack_compute_floatingip_v2" "master" {
  count = var.floating_ip_network == "" ? 0 : 1
  pool = var.floating_ip_network
}

resource "openstack_compute_floatingip_associate_v2" "master" {
  count = var.floating_ip_network == "" ? 0 : 1
  floating_ip = openstack_compute_floatingip_v2.master[0].address
  instance_id = openstack_compute_instance_v2.master.id
}

