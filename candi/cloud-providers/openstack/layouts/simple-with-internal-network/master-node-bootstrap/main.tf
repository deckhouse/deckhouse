locals {
  root_disk_size = lookup(var.providerInitConfig.masterInstanceClass, "rootDiskSizeInGb", "")
  image_name = var.providerInitConfig.masterInstanceClass.imageName
  flavor_name = var.providerInitConfig.masterInstanceClass.flavorName
  security_group_names = lookup(var.providerInitConfig.masterInstanceClass, "securityGroups", [])
  network_security = local.pod_network_mode == "DirectRoutingWithPortSecurityEnabled"
}

module "simple_master" {
  source = "../../../terraform-modules/simple-master-with-internal-network"
  prefix = local.prefix
  root_disk_size = local.root_disk_size
  image_name = local.image_name
  flavor_name = local.flavor_name
  keypair_ssh_name = data.openstack_compute_keypair_v2.ssh.name
  master_internal_port_id = local.network_security ? openstack_networking_port_v2.master_internal_with_security[0].id : openstack_networking_port_v2.master_internal_without_security[0].id
}

module "kubernetes_data" {
  source = "../../../terraform-modules/kubernetes-data"
  prefix = local.prefix
  master_id = module.simple_master.id
}

module "security_groups" {
  source = "../../../terraform-modules/security-groups"
  security_group_names = local.security_group_names
}

data "openstack_compute_keypair_v2" "ssh" {
  name = local.prefix
}

data "openstack_networking_network_v2" "external" {
  count = local.external_network_name == "" ? 0 : 1
  name = local.external_network_name
}

data "openstack_networking_subnet_v2" "internal" {
  name = local.internal_subnet_name
}

data "openstack_networking_network_v2" "internal" {
  network_id = data.openstack_networking_subnet_v2.internal.network_id
}

resource "openstack_networking_port_v2" "master_internal_with_security" {
  count = local.network_security ? 1 : 0
  network_id = data.openstack_networking_network_v2.internal.id
  admin_state_up = "true"
  security_group_ids = module.security_groups.security_group_ids
  fixed_ip {
    subnet_id = data.openstack_networking_subnet_v2.internal.id
  }
  allowed_address_pairs {
    ip_address = local.pod_subnet_cidr
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
