# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

resource "vcd_network_routed_v2" "network" {
  org  = var.providerClusterConfiguration.organization
  name = var.providerClusterConfiguration.mainNetwork

  edge_gateway_id = var.edgeGatewayId

  gateway       = cidrhost(var.providerClusterConfiguration.internalNetworkCIDR, 1)
  prefix_length = tonumber(split("/", var.providerClusterConfiguration.internalNetworkCIDR)[1])
  dns1          = length(var.providerClusterConfiguration.internalNetworkDNSServers) > 0 ? var.providerClusterConfiguration.internalNetworkDNSServers[0] : null
  dns2          = length(var.providerClusterConfiguration.internalNetworkDNSServers) > 1 ? var.providerClusterConfiguration.internalNetworkDNSServers[1] : null
}

resource "vcd_nsxt_network_dhcp" "pools" {
  count = var.useNSXV ? 0 : 1

  org = var.providerClusterConfiguration.organization
  org_network_id = vcd_network_routed_v2.network.id
  dns_servers    = var.providerClusterConfiguration.internalNetworkDNSServers

  pool {
    start_address = cidrhost(var.providerClusterConfiguration.internalNetworkCIDR, 30)
    end_address   = cidrhost(var.providerClusterConfiguration.internalNetworkCIDR, -2)
  }

}
