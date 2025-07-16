# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  isFirstNode = var.nodeIndex == 0
}

# NSX-T DNAT rule only for the first master node

data "vcd_nsxt_app_port_profile" "ssh" {
  count = local.isFirstNode ? (var.useNSXV ? 0 : 1) : 0
  org   = var.providerClusterConfiguration.organization
  name  = "SSH"
  scope = "SYSTEM"
}

resource "vcd_nsxt_nat_rule" "master-dnat" {
  count = local.isFirstNode && var.useNSXV ? 0 : 1
  org   = var.providerClusterConfiguration.organization

  edge_gateway_id = var.edgeGatewayId

  name        = format("%s-dnat-ssh", var.providerClusterConfiguration.mainNetwork)
  rule_type   = "DNAT"
  description = format("SSH DNAT rule for first master of %s", var.providerClusterConfiguration.virtualApplicationName)

  external_address    = var.providerClusterConfiguration.edgeGateway.externalIP
  dnat_external_port  = var.providerClusterConfiguration.edgeGateway.externalPort
  internal_address    = var.node_ip
  logging             = false
  app_port_profile_id = data.vcd_nsxt_app_port_profile.ssh[0].id
}

# NSX-V DNAT rule only for the first master node

resource "vcd_nsxv_dnat" "master-dnat" {
  count = local.isFirstNode && var.useNSXV ? 1 : 0

  enabled     = true
  description = format("SSH DNAT rule for first master of %s", var.providerClusterConfiguration.virtualApplicationName)
  org         = var.providerClusterConfiguration.organization

  edge_gateway = var.providerClusterConfiguration.edgeGateway.name
  network_type = var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkType
  network_name = var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkName

  original_address   = var.providerClusterConfiguration.edgeGateway.externalIP
  original_port      = var.providerClusterConfiguration.edgeGateway.externalPort
  translated_address = var.node_ip
  protocol           = "tcp"
  translated_port    = 22
}
