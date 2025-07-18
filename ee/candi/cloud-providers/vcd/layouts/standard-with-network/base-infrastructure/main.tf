# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  create_default_firewall_rules            = contains(keys(var.providerClusterConfiguration), "createDefaultFirewallRules") ? var.providerClusterConfiguration.createDefaultFirewallRules : false
  external_network_name                    = contains(keys(var.providerClusterConfiguration.edgeGateway), "NSX-V") ? var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkName : null
  external_network_type                    = contains(keys(var.providerClusterConfiguration.edgeGateway), "NSX-V") ? var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkType : null
  internal_network_dhcp_pool_start_address = contains(keys(var.providerClusterConfiguration), "internalNetworkDHCPPoolStartAddress") ? var.providerClusterConfiguration.internalNetworkDHCPPoolStartAddress : 30
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

module "firewall" {
  count = local.create_default_firewall_rules ? 1 : 0

  source                = "../../../terraform-modules/firewall"
  organization          = var.providerClusterConfiguration.organization
  edge_gateway_name     = var.providerClusterConfiguration.edgeGateway.name
  edge_gateway_type     = var.providerClusterConfiguration.edgeGateway.type
  internal_network_name = var.providerClusterConfiguration.mainNetwork
  internal_network_cidr = var.providerClusterConfiguration.internalNetworkCIDR
}
