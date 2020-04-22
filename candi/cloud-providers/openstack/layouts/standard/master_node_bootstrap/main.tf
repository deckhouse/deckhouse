locals {
  root_disk_size = var.providerInitConfig.masterInstanceClass.rootDiskSizeInGb
  image_name = var.providerInitConfig.masterInstanceClass.imageName
  flavor_name = var.providerInitConfig.masterInstanceClass.flavorName
}

module "network_security_info" {
  source = "../../../terraform_modules//network_security_info"
  prefix = local.prefix
  enabled = local.network_security
}

module "standard_master" {
  source = "../../../terraform_modules//standard_master"
  prefix = local.prefix
  root_disk_size = local.root_disk_size
  image_name = local.image_name
  flavor_name = local.flavor_name
  keypair_ssh_name = data.openstack_compute_keypair_v2.ssh.name
  master_internal_port_id = local.network_security ? openstack_networking_port_v2.master_internal_with_security[0].id : openstack_networking_port_v2.master_internal_without_security[0].id
  external_network_name = data.openstack_networking_network_v2.external.name
  internal_subnet = data.openstack_networking_subnet_v2.internal
}

module "kubernetes_data" {
  source = "../../../terraform_modules/kubernetes_data"
  prefix = local.prefix
  master_id = module.standard_master.id
}

data "openstack_compute_keypair_v2" "ssh" {
  name = local.prefix
}

data "openstack_networking_network_v2" "external" {
  name = local.external_network_name
}

data "openstack_networking_network_v2" "internal" {
  name = local.prefix
}

data "openstack_networking_subnet_v2" "internal" {
  name = local.prefix
}

resource "openstack_networking_port_v2" "master_internal_with_security" {
  count = local.network_security ? 1 : 0
  network_id = data.openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  security_group_ids = module.network_security_info.security_group_ids
  fixed_ip {
    subnet_id = data.openstack_networking_subnet_v2.internal.id
  }
  allowed_address_pairs {
    ip_address = "10.244.0.0/16"
  }
}

resource "openstack_networking_port_v2" "master_internal_without_security" {
  count = local.network_security ? 0 : 1
  network_id = data.openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  fixed_ip {
    subnet_id = data.openstack_networking_subnet_v2.internal.id
  }
}
