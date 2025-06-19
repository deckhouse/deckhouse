# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  use_nsxt = var.providerClusterConfiguration.edgeGatewayType == "NSX-T"
}

data "vcd_nsxt_edgegateway" "gateway" {
  count = local.use_nsxt ? 1 : 0
  org   = var.providerClusterConfiguration.organization
  name  = var.providerClusterConfiguration.edgeGatewayName
}

data "vcd_edgegateway" "gateway" {
  count = local.use_nsxt ? 0 : 1
  org   = var.providerClusterConfiguration.organization
  name  = var.providerClusterConfiguration.edgeGatewayName
}

resource "vcd_network_routed_v2" "network" {
  org  = var.providerClusterConfiguration.organization
  name = var.providerClusterConfiguration.mainNetwork

  edge_gateway_id = local.use_nsxt ? data.vcd_nsxt_edgegateway.gateway[0].id : data.vcd_edgegateway.gateway[0].id

  gateway       = cidrhost(var.providerClusterConfiguration.internalNetworkCIDR, 1)
  prefix_length = tonumber(split("/", var.providerClusterConfiguration.internalNetworkCIDR)[1])
  dns1          = length(var.providerClusterConfiguration.internalNetworkDNSServers) > 0 ? var.providerClusterConfiguration.internalNetworkDNSServers[0] : null
  dns2          = length(var.providerClusterConfiguration.internalNetworkDNSServers) > 1 ? var.providerClusterConfiguration.internalNetworkDNSServers[1] : null
}
