# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

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

module "node" {
  source                = "../../../terraform-modules/static-node"
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
  node_group_name       = var.nodeGroupName
  subnet                = local.prefix
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
