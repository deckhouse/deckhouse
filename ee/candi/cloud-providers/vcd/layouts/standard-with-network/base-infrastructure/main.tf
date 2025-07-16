# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  createDefaultFirewallRules = contains(keys(var.providerClusterConfiguration), "createDefaultFirewallRules") ? var.providerClusterConfiguration.createDefaultFirewallRules : false
}

module "network" {
  source                                   = "../../../terraform-modules/network"
  organization                             = var.providerClusterConfiguration.organization
  edge_gateway_name                        = var.providerClusterConfiguration.edgeGateway.name
  edge_gateway_type                        = var.providerClusterConfiguration.edgeGateway.type
  internal_network_name                    = var.providerClusterConfiguration.mainNetwork
  internal_network_cidr                    = var.providerClusterConfiguration.internalNetworkCIDR
  internal_network_dhcp_pool_start_address = var.providerClusterConfiguration.internalNetworkDHCPPoolStartAddress
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
  external_network_name = var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkName
  external_network_type = var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkType
  external_address      = var.providerClusterConfiguration.edgeGateway.externalIP
  external_port         = var.providerClusterConfiguration.edgeGateway.externalPort
}

module "firewall" {
  count = local.createDefaultFirewallRules ? 1 : 0

  source                       = "../../../terraform-modules/firewall"
  organization          = var.providerClusterConfiguration.organization
  edge_gateway_name     = var.providerClusterConfiguration.edgeGateway.name
  edge_gateway_type     = var.providerClusterConfiguration.edgeGateway.type
  internal_network_name = var.providerClusterConfiguration.mainNetwork
  internal_network_cidr = var.providerClusterConfiguration.internalNetworkCIDR
}
