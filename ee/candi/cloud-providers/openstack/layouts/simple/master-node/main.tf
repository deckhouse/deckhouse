# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  security_group_names = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalSecurityGroups", [])
  volume_type_map = var.providerClusterConfiguration.masterNodeGroup.volumeTypeMap
  actual_zones = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.openstack_compute_availability_zones_v2.zones.names, var.providerClusterConfiguration.zones)) : data.openstack_compute_availability_zones_v2.zones.names
  zone = element(tolist(setintersection(keys(local.volume_type_map), local.actual_zones)), var.nodeIndex)
  volume_type = local.volume_type_map[local.zone]
  flavor_name = var.providerClusterConfiguration.masterNodeGroup.instanceClass.flavorName
  root_disk_size = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "rootDiskSize", "") # Openstack can have disks predefined within vm flavours, so we do not set any defaults here
  etcd_volume_size = var.providerClusterConfiguration.masterNodeGroup.instanceClass.etcdDiskSizeGb
  additional_tags = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalTags", {})
}

module "security_groups" {
  source = "../../../terraform-modules/security-groups"
  security_group_names = local.security_group_names
}

data "openstack_compute_availability_zones_v2" "zones" {}

data "openstack_images_image_v2" "master" {
  name = local.image_name
}

data "openstack_compute_keypair_v2" "ssh" {
  name = local.prefix
}

data "openstack_networking_network_v2" "external" {
  name = local.external_network_name
}

module "volume_zone" {
  source       = "../../../terraform-modules/volume-zone"
  compute_zone = local.zone
  region       = var.providerClusterConfiguration.provider.region
}

module "kubernetes_data" {
  source = "../../../terraform-modules/kubernetes-data"
  prefix = local.prefix
  node_index = var.nodeIndex
  master_id = openstack_compute_instance_v2.master.id
  volume_size = local.etcd_volume_size
  volume_type = local.volume_type
  volume_zone = module.volume_zone.zone
  tags = local.tags
}

locals {
  metadata_tags = merge(local.tags, local.additional_tags)
}

resource "openstack_blockstorage_volume_v3" "master" {
  count = local.root_disk_size == "" ? 0 : 1
  name = join("-", [local.prefix, "master-root-volume", var.nodeIndex])
  size = local.root_disk_size
  image_id = data.openstack_images_image_v2.master.id
  metadata = length(local.metadata_tags) > 0 ? local.metadata_tags : null
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

resource "openstack_compute_instance_v2" "master" {
  name = join("-", [local.prefix, "master", var.nodeIndex])
  image_name = data.openstack_images_image_v2.master.name
  flavor_name = local.flavor_name
  key_pair = data.openstack_compute_keypair_v2.ssh.name
  config_drive = !local.external_network_dhcp
  user_data = var.cloudConfig == "" ? null : base64decode(var.cloudConfig)
  availability_zone = local.zone
  security_groups = local.security_group_names

  network {
    name = local.external_network_name
  }

  dynamic "block_device" {
    for_each = local.root_disk_size == "" ? [] : list(openstack_blockstorage_volume_v3.master[0])
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

  timeouts {
    create = var.resourceManagementTimeout
    delete = var.resourceManagementTimeout
    update = var.resourceManagementTimeout
  }

  metadata = length(local.metadata_tags) > 0 ? local.metadata_tags : null
}
