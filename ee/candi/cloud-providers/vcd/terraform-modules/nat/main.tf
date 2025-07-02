# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  main_ip_addresses = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "mainNetworkIPAddresses", [])
}

resource "vcd_nsxt_nat_rule" "snat" {
  count = var.useNSXV ? 0 : 1
  org   = var.providerClusterConfiguration.organization

  edge_gateway_id = var.edgeGatewayId

  name        = format("%s-snat", var.providerClusterConfiguration.mainNetwork)
  rule_type   = "SNAT"
  description = format("SNAT rule for %s", var.providerClusterConfiguration.mainNetwork)

  external_address = var.providerClusterConfiguration.edgeGateway.externalIP
  internal_address = var.providerClusterConfiguration.internalNetworkCIDR
  logging          = false
}

data "vcd_nsxt_app_port_profile" "ssh" {
  count = var.useNSXV ? 0 : 1
  org   = var.providerClusterConfiguration.organization
  name  = "SSH"
  scope = "SYSTEM"
}

resource "vcd_nsxt_nat_rule" "master-dnat" {
  count = (!var.useNSXV && length(local.main_ip_addresses) > 0) ? 1 : 0
  org   = var.providerClusterConfiguration.organization

  edge_gateway_id = var.edgeGatewayId

  name        = format("%s-dnat-ssh", var.providerClusterConfiguration.mainNetwork)
  rule_type   = "DNAT"
  description = format("SSH DNAT rule for first master of %s", var.providerClusterConfiguration.virtualApplicationName)

  external_address    = var.providerClusterConfiguration.edgeGateway.externalIP
  dnat_external_port  = var.providerClusterConfiguration.edgeGateway.externalPort
  internal_address    = local.main_ip_addresses[count.index]
  logging             = false
  app_port_profile_id = data.vcd_nsxt_app_port_profile.ssh[0].id
}

resource "vcd_nsxv_snat" "snat" {
  count = var.useNSXV ? 1 : 0

  enabled     = true
  description = format("SNAT rule for %s", var.providerClusterConfiguration.mainNetwork)
  org         = var.providerClusterConfiguration.organization

  edge_gateway = var.providerClusterConfiguration.edgeGateway.name
  network_type = var.providerClusterConfiguration.edgeGateway.externalNetworkType
  network_name = var.providerClusterConfiguration.edgeGateway.externalNetworkName

  original_address   = var.providerClusterConfiguration.internalNetworkCIDR
  translated_address = var.providerClusterConfiguration.edgeGateway.externalIP
}


resource "vcd_nsxv_dnat" "master-dnat" {
  count = var.useNSXV ? 1 : 0

  enabled     = true
  description = format("SSH DNAT rule for first master of %s", var.providerClusterConfiguration.virtualApplicationName)
  org         = var.providerClusterConfiguration.organization

  edge_gateway = var.providerClusterConfiguration.edgeGateway.name
  network_type = var.providerClusterConfiguration.edgeGateway.externalNetworkType
  network_name = var.providerClusterConfiguration.edgeGateway.externalNetworkName

  original_address   = var.providerClusterConfiguration.edgeGateway.externalIP
  original_port      = var.providerClusterConfiguration.edgeGateway.externalPort
  translated_address = local.main_ip_addresses[0]
  protocol           = "tcp"
  translated_port    = 22
}
