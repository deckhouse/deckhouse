# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "vcd_nsxt_edgegateway" "gateway" {
  org  = var.providerClusterConfiguration.organization
  name = var.providerClusterConfiguration.edgeGatewayName
}

resource "vcd_network_routed_v2" "network" {
  org  = var.providerClusterConfiguration.virtualApplicationName
  name = var.prefix

  edge_gateway_id = data.vcd_nsxt_edgegateway.edge.id

  gateway            = cidrhost(var.providerClusterConfiguration.internalNetworkCIDR, 1)
  prefix_length      = 24
  guest_vlan_allowed = false
  dns1               = "8.8.8.8"
  dns2               = "1.1.1.1"
}
