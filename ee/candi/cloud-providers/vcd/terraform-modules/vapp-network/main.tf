# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "vcd_network" "parent" {
  org  = var.providerClusterConfiguration.organization
  vdc  = var.providerClusterConfiguration.virtualDataCenter
  name = var.providerClusterConfiguration.mainNetwork
}

resource "vcd_vapp_network_routed" "network" {
  org                = var.providerClusterConfiguration.organization
  vdc                = var.providerClusterConfiguration.virtualDataCenter
  name               = var.providerClusterConfiguration.prefix
  parent_network_id  = data.vcd_network.parent.id
  gateway             = cidrhost(var.providerClusterConfiguration.internalNetworkCIDR, 1)
  prefix_length      = tonumber(split("/", var.providerClusterConfiguration.internalNetworkCIDR)[1])
  dns1               = length(var.providerClusterConfiguration.internalNetworkDNSServers) > 0 ? var.providerClusterConfiguration.internalNetworkDNSServers[0] : null
  dns2               = length(var.providerClusterConfiguration.internalNetworkDNSServers) > 1 ? var.providerClusterConfiguration.internalNetworkDNSServers[1] : null
  retain_ip_mac_enabled = true
}
