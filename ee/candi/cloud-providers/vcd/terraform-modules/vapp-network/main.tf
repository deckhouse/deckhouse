# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

resource "vcd_vapp_network" "network" {
  org                   = var.providerClusterConfiguration.organization
  vapp_name             = var.providerClusterConfiguration.virtualApplicationName
  name                  = var.vappName
  org_network_name      = var.providerClusterConfiguration.mainNetwork
  gateway               = cidrhost(var.providerClusterConfiguration.internalNetworkCIDR, 1)
  prefix_length         = tonumber(split("/", var.providerClusterConfiguration.internalNetworkCIDR)[1])
  dns1                  = length(var.providerClusterConfiguration.internalNetworkDNSServers) > 0 ? var.providerClusterConfiguration.internalNetworkDNSServers[0] : null
  dns2                  = length(var.providerClusterConfiguration.internalNetworkDNSServers) > 1 ? var.providerClusterConfiguration.internalNetworkDNSServers[1] : null
  retain_ip_mac_enabled = true
}
