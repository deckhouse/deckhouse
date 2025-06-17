# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "vcd_nsxt_edgegateway" "gateway" {
  org  = var.providerClusterConfiguration.organization
  name = var.providerClusterConfiguration.edgeGatewayName
}

resource "vcd_network_routed_v2" "network" {
  org  = var.providerClusterConfiguration.organization
  name = var.providerClusterConfiguration.mainNetwork

  edge_gateway_id = data.vcd_nsxt_edgegateway.gateway.id

  gateway            = cidrhost(var.providerClusterConfiguration.internalNetworkCIDR, 1)
  prefix_length      = tonumber(split("/", var.providerClusterConfiguration.internalNetworkCIDR)[1])
  guest_vlan_allowed = false
  dns1               = length(var.providerClusterConfiguration.internalNetworkDNSServers) > 0 ? var.providerClusterConfiguration.internalNetworkDNSServers[0] : null
  dns2               = length(var.providerClusterConfiguration.internalNetworkDNSServers) > 1 ? var.providerClusterConfiguration.internalNetworkDNSServers[1] : null
}
