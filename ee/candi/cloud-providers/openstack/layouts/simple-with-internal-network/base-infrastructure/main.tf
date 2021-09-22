# Copyright 2021 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

module "keypair" {
  source = "../../../terraform-modules/keypair"
  prefix = local.prefix
  ssh_public_key = var.providerClusterConfiguration.sshPublicKey
}

data "openstack_compute_availability_zones_v2" "zones" {}

data "openstack_networking_subnet_v2" "internal" {
  name = local.internal_subnet_name
}

data "openstack_networking_network_v2" "internal" {
  network_id = data.openstack_networking_subnet_v2.internal.network_id
}

data "openstack_networking_network_v2" "external" {
  count = local.external_network_name == "" ? 0 : 1
  name = local.external_network_name
}
