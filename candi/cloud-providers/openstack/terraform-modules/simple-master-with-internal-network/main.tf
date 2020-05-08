data "openstack_images_image_v2" "master" {
  name = var.image_name
}

resource "openstack_blockstorage_volume_v2" "master" {
  count = var.root_disk_size == "" ? 0 : 1
  name = join("-", [var.prefix, "master-root-volume"])
  size = var.root_disk_size
  image_id = data.openstack_images_image_v2.master.id
}

resource "openstack_compute_instance_v2" "master" {
  count = var.root_disk_size == "" ? 1 : 0
  name = join("-", [var.prefix, "master"])
  image_name = data.openstack_images_image_v2.master.name
  flavor_name = var.flavor_name
  key_pair = var.keypair_ssh_name

  network {
    port = var.master_internal_port_id
  }
}

resource "openstack_compute_instance_v2" "master_with_root_disk" {
  count = var.root_disk_size == "" ? 0 : 1
  name = join("-", [var.prefix, "master"])
  image_name = data.openstack_images_image_v2.master.name
  flavor_name = var.flavor_name
  key_pair = var.keypair_ssh_name

  network {
    port = var.master_internal_port_id
  }

  block_device {
    uuid                  = openstack_blockstorage_volume_v2.master[0].id
    boot_index            = 0
    source_type           = "volume"
    destination_type      = "volume"
    delete_on_termination = true
  }
}
