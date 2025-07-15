# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  main_ip_addresses = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "mainNetworkIPAddresses", [])
}

# NSX-T resources

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

# NSX-V resources

resource "vcd_nsxv_snat" "snat" {
  count = var.useNSXV ? 1 : 0

  enabled     = true
  description = format("SNAT rule for %s", var.providerClusterConfiguration.mainNetwork)
  org         = var.providerClusterConfiguration.organization

  edge_gateway = var.providerClusterConfiguration.edgeGateway.name
  network_type = var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkType
  network_name = var.providerClusterConfiguration.edgeGateway.NSX-V.externalNetworkName

  original_address   = var.providerClusterConfiguration.internalNetworkCIDR
  translated_address = var.providerClusterConfiguration.edgeGateway.externalIP
}
