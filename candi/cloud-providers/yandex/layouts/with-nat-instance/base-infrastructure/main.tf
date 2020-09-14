resource "yandex_vpc_network" "kube" {
  count = local.existing_network_id != "" ? 0 : 1
  name = local.prefix
}

locals {
  network_id = local.existing_network_id != "" ? local.existing_network_id : join("", yandex_vpc_network.kube.*.id) # https://github.com/hashicorp/terraform/issues/23222#issuecomment-547462883
}


module "vpc_components" {
  source = "../../../terraform-modules/vpc-components"
  prefix = local.prefix
  network_id = local.network_id
  node_network_cidr = local.node_network_cidr
  dhcp_domain_name = local.dhcp_domain_name
  dhcp_domain_name_servers = local.dhcp_domain_name_servers
  should_create_nat_instance = true
  nat_instance_external_address = local.nat_instance_external_address
  nat_instance_internal_subnet_id = local.nat_instance_internal_subnet_id
  nat_instance_external_subnet_id = local.nat_instance_external_subnet_id
  nat_instance_ssh_key = var.providerClusterConfiguration.sshPublicKey
}
