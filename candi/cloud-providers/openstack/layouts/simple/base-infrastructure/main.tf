module "keypair" {
  source = "../../../terraform-modules/keypair"
  prefix = local.prefix
  ssh_public_key = var.providerClusterConfiguration.sshPublicKey
}

data "openstack_compute_availability_zones_v2" "zones" {}

data "openstack_networking_network_v2" "external" {
  name = local.external_network_name
}
