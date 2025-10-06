# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  security_group_names = local.network_security ? concat([local.prefix], lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalSecurityGroups", [])) : []
  volume_type_map      = var.providerClusterConfiguration.masterNodeGroup.volumeTypeMap
  actual_zones         = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.huaweicloud_availability_zones.zones.names, var.providerClusterConfiguration.zones)) : data.huaweicloud_availability_zones.zones.names
  zone                 = element(tolist(setintersection(keys(local.volume_type_map), local.actual_zones)), var.nodeIndex)
  volume_type          = local.volume_type_map[local.zone]
  flavor_name          = var.providerClusterConfiguration.masterNodeGroup.instanceClass.flavorName
  root_disk_size       = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "rootDiskSize", 10) # Huaweicloud can have disks predefined within vm flavours, so we do not set any defaults here
  etcd_volume_size     = var.providerClusterConfiguration.masterNodeGroup.instanceClass.etcdDiskSizeGb
  additional_tags      = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "additionalTags", {})
  subnet               = var.providerClusterConfiguration.vpcPeering.subnet
}

module "network_security_info" {
  source  = "../../../terraform-modules/network-security-info"
  prefix  = local.prefix
  enabled = local.network_security
}

module "volume_zone" {
  source       = "../../../terraform-modules/volume-zone"
  compute_zone = local.zone
  region       = var.providerClusterConfiguration.provider.region
}

module "master" {
  source                = "../../../terraform-modules/master"
  prefix                = local.prefix
  node_index            = var.nodeIndex
  cloud_config          = var.cloudConfig
  flavor_name           = local.flavor_name
  root_disk_size        = local.root_disk_size
  additional_tags       = local.additional_tags
  image_name            = local.image_name
  keypair_ssh_name      = data.huaweicloud_kps_keypairs.ssh.name
  security_group_ids    = module.security_groups.security_group_ids
  internal_network_cidr = local.internal_network_cidr
  enable_eip            = false
  tags                  = local.tags
  zone                  = local.zone
  volume_type           = local.volume_type
  volume_zone           = module.volume_zone.zone
  server_group          = local.server_group
  subnet                = local.subnet
  enterprise_project_id = local.enterprise_project_id
}

module "kubernetes_data" {
  source      = "../../../terraform-modules/kubernetes-data"
  prefix      = local.prefix
  node_index  = var.nodeIndex
  master_id   = module.master.id
  volume_size = local.etcd_volume_size
  volume_type = local.volume_type
  volume_zone = module.volume_zone.zone
  tags        = local.tags
  enterprise_project_id = local.enterprise_project_id
}

module "security_groups" {
  source                      = "../../../terraform-modules/security-groups"
  security_group_names        = local.security_group_names
  layout_security_group_ids   = module.network_security_info.security_group_ids
  layout_security_group_names = module.network_security_info.security_group_names
  enterprise_project_id = local.enterprise_project_id
}

data "huaweicloud_availability_zones" "zones" {}

data "huaweicloud_kps_keypairs" "ssh" {
  name = local.prefix
}
