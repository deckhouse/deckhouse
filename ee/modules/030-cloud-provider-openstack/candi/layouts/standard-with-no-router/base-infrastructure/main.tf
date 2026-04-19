# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

module "network_security" {
  source            = "../../../terraform-modules/network-security"
  prefix            = local.prefix
  ssh_allow_list    = local.ssh_allow_list
  enabled           = local.network_security
}

module "keypair" {
  source = "../../../terraform-modules/keypair"
  prefix = local.prefix
  ssh_public_key = var.providerClusterConfiguration.sshPublicKey
}

data "openstack_compute_availability_zones_v2" "zones" {}

data "openstack_networking_network_v2" "external" {
  name = local.external_network_name
}

resource "openstack_networking_network_v2" "internal" {
  name = local.prefix
  admin_state_up = "true"
}

resource "openstack_networking_subnet_v2" "internal" {
  name = local.prefix
  network_id = openstack_networking_network_v2.internal.id
  cidr = local.internal_network_cidr
  ip_version = 4
  no_gateway = "true"
}
