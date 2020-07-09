module "keypair" {
  source = "../../../terraform-modules/keypair"
  prefix = local.prefix
  ssh_public_key = var.clusterConfiguration.sshPublicKeys[0]
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
