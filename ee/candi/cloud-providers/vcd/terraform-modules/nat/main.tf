# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  useNSXT       = var.providerClusterConfiguration.edgeGatewayType == "NSX-T"
  edgeGatewayId = local.useNSXT ? data.vcd_nsxt_edgegateway.gateway[0].id : data.vcd_edgegateway.gateway[0].id
  main_ip_addresses = lookup(var.providerClusterConfiguration.masterNodeGroup.instanceClass, "mainNetworkIPAddresses", [])
}

resource "vcd_nsxt_nat_rule" "snat" {
  count = local.useNSXT ? 1 : 0
  org = var.providerClusterConfiguration.organization

  edge_gateway_id = local.edgeGatewayId

  name        = format("%-snat", var.providerClusterConfiguration.mainNetwork)
  rule_type   = "SNAT"
  description = format("SNAT rule for %s", var.providerClusterConfiguration.mainNetwork)

  external_address = var.providerClusterConfiguration.edgeGatewayExternalIP
  internal_address = var.providerClusterConfiguration.internalNetworkCIDR
  logging          = false
}

data "vcd_nsxt_app_port_profile" "ssh" {
  count = local.useNSXT ? 1 : 0
  org   = var.providerClusterConfiguration.organization
  name  = "SSH"
  scope = "SYSTEM"
}

resource "vcd_nsxt_nat_rule" "masters-dnat" {
  count = local.useNSXT ? 1 : 0 && length(local.main_ip_addresses) > 0
  org = var.providerClusterConfiguration.organization

  edge_gateway_id = local.edgeGatewayId

  name        = format("%s-dnat-ssh", var.providerClusterConfiguration.mainNetwork)
  rule_type   = "DNAT"
  description = format("DNAT rule for %s", var.providerClusterConfiguration.mainNetwork)

  external_address    = var.providerClusterConfiguration.edgeGatewayExternalIP
  dnat_external_port  = var.providerClusterConfiguration.edgeGatewayExternalPort
  internal_address    = local.main_ip_addresses[count.index]
  logging             = false
  app_port_profile_id = data.vcd_nsxt_app_port_profile.ssh.id
}
