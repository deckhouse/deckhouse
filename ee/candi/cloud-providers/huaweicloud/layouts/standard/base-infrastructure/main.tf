# Copyright 2024 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  volume_type_map = var.providerClusterConfiguration.masterNodeGroup.volumeTypeMap
  actual_zones    = lookup(var.providerClusterConfiguration, "zones", null) != null ? tolist(setintersection(data.huaweicloud_availability_zones.zones.names, var.providerClusterConfiguration.zones)) : data.huaweicloud_availability_zones.zones.names
  zone            = element(tolist(setintersection(keys(local.volume_type_map), local.actual_zones)), 0)
}

module "network_security" {
  source         = "../../../terraform-modules/network-security"
  prefix         = local.prefix
  ssh_allow_list = local.ssh_allow_list
  enabled        = local.network_security
}

module "keypair" {
  source         = "../../../terraform-modules/keypair"
  prefix         = local.prefix
  ssh_public_key = var.providerClusterConfiguration.sshPublicKey
}

data "huaweicloud_availability_zones" "zones" {}

resource "huaweicloud_vpc" "vpc" {
  name = local.prefix
  cidr = local.internal_network_cidr
}

resource "huaweicloud_vpc_subnet" "subnet" {
  name              = local.prefix
  cidr              = local.internal_network_cidr
  gateway_ip        = cidrhost(local.internal_network_cidr, 1)
  vpc_id            = huaweicloud_vpc.vpc.id
  availability_zone = local.zone
  dhcp_enable       = true
  dns_list          = lookup(var.providerClusterConfiguration.standard, "internalNetworkDNSServers", [])
}

resource "huaweicloud_compute_servergroup" "server_group" {
  count    = local.server_group_policy == "AntiAffinity" ? 1 : 0
  name     = local.prefix
  policies = ["anti-affinity"]
}
