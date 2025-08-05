# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  create_default_firewall_rules            = contains(keys(var.providerClusterConfiguration), "createDefaultFirewallRules") ? var.providerClusterConfiguration.createDefaultFirewallRules : false
  external_network_name                    = contains(keys(var.providerClusterConfiguration.edgeGateway), "NSX-V") ? var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkName : null
  external_network_type                    = contains(keys(var.providerClusterConfiguration.edgeGateway), "NSX-V") ? var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkType : null
  internal_network_dhcp_pool_start_address = contains(keys(var.providerClusterConfiguration), "internalNetworkDHCPPoolStartAddress") ? var.providerClusterConfiguration.internalNetworkDHCPPoolStartAddress : 30
  bastion_placement_policy                 = contains(keys(var.providerClusterConfiguration.bastion), "placementPolicy") ? var.providerClusterConfiguration.bastion.placementPolicy : ""
}

module "network" {
  source                                   = "../../../terraform-modules/network"
  organization                             = var.providerClusterConfiguration.organization
  edge_gateway_name                        = var.providerClusterConfiguration.edgeGateway.name
  edge_gateway_type                        = var.providerClusterConfiguration.edgeGateway.type
  internal_network_name                    = var.providerClusterConfiguration.mainNetwork
  internal_network_cidr                    = var.providerClusterConfiguration.internalNetworkCIDR
  internal_network_dhcp_pool_start_address = local.internal_network_dhcp_pool_start_address
  internal_network_dns_servers             = var.providerClusterConfiguration.internalNetworkDNSServers
}

module "vapp" {
  source       = "../../../terraform-modules/vapp"
  organization = var.providerClusterConfiguration.organization
  vapp_name    = var.providerClusterConfiguration.virtualApplicationName
}

resource "vcd_vapp_org_network" "vapp_network" {
  org                    = var.providerClusterConfiguration.organization
  vdc                    = var.providerClusterConfiguration.virtualDataCenter
  vapp_name              = module.vapp.name
  org_network_name       = module.network.name
  reboot_vapp_on_removal = true
}

module "bastion" {
  source            = "../../../terraform-modules/bastion"
  organization      = var.providerClusterConfiguration.organization
  vdc_name          = var.providerClusterConfiguration.virtualDataCenter
  prefix            = var.clusterConfiguration.cloud.prefix
  vapp_name         = module.vapp.name
  network_name      = vcd_vapp_org_network.vapp_network.org_network_name
  ip_address        = var.providerClusterConfiguration.bastion.mainNetworkIPAddress
  template          = var.providerClusterConfiguration.bastion.template
  ssh_public_key    = var.providerClusterConfiguration.sshPublicKey
  placement_policy  = local.bastion_placement_policy
  storage_profile   = var.providerClusterConfiguration.bastion.storageProfile
  sizing_policy     = var.providerClusterConfiguration.bastion.sizingPolicy
  root_disk_size_gb = var.providerClusterConfiguration.bastion.rootDiskSizeGb
}

module "snat" {
  source                = "../../../terraform-modules/snat"
  organization          = var.providerClusterConfiguration.organization
  edge_gateway_name     = var.providerClusterConfiguration.edgeGateway.name
  edge_gateway_type     = var.providerClusterConfiguration.edgeGateway.type
  internal_network_name = var.providerClusterConfiguration.mainNetwork
  internal_network_cidr = var.providerClusterConfiguration.internalNetworkCIDR
  external_network_name = local.external_network_name
  external_network_type = local.external_network_type
  external_address      = var.providerClusterConfiguration.edgeGateway.externalIP
  external_port         = var.providerClusterConfiguration.edgeGateway.externalPort
}

module "dnat_bastion" {
  source                = "../../../terraform-modules/dnat"
  organization          = var.providerClusterConfiguration.organization
  rule_name_prefix      = format("%s-bastion", var.clusterConfiguration.cloud.prefix)
  rule_description      = format("SSH DNAT rule for bastion of %s", var.providerClusterConfiguration.virtualApplicationName)
  edge_gateway_name     = var.providerClusterConfiguration.edgeGateway.name
  edge_gateway_type     = var.providerClusterConfiguration.edgeGateway.type
  internal_network_name = var.providerClusterConfiguration.mainNetwork
  internal_address      = module.bastion.bastion_ip_address_for_ssh
  external_address      = var.providerClusterConfiguration.edgeGateway.externalIP
  external_port         = var.providerClusterConfiguration.edgeGateway.externalPort
  external_network_name = local.external_network_name
  external_network_type = local.external_network_type
}


module "firewall" {
  count = local.create_default_firewall_rules ? 1 : 0

  source                = "../../../terraform-modules/firewall"
  organization          = var.providerClusterConfiguration.organization
  edge_gateway_name     = var.providerClusterConfiguration.edgeGateway.name
  edge_gateway_type     = var.providerClusterConfiguration.edgeGateway.type
  internal_network_name = var.providerClusterConfiguration.mainNetwork
  internal_network_cidr = var.providerClusterConfiguration.internalNetworkCIDR
}
