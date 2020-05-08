module "network_security" {
  source = "../../../terraform-modules/network-security"
  prefix = local.prefix
  remote_ip_prefix = "0.0.0.0/0"
  enabled = local.network_security
}

module "keypair" {
  source = "../../../terraform-modules/keypair"
  prefix = local.prefix
  ssh_public_key = var.initConfig.sshPublicKeys[0]
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
