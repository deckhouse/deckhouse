# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

data "vcd_vdc_group" "vdc" {
  org  = var.providerClusterConfiguration.organization
  name = var.providerClusterConfiguration.virtualDataCenter
}

data "vcd_nsxt_app_port_profile" "ssh" {
  context_id = data.vcd_nsxt_manager.main.id
  name       = "ssh"
  scope      = "SYSTEM"
}

resource "vcd_nsxt_security_group" "network" {
  org = var.providerClusterConfiguration.organization
  edge_gateway_id = var.edgeGatewayId

  name        = var.providerClusterConfiguration.mainNetwork
  description = format("%s members", var.providerClusterConfiguration.mainNetwork)

  member_org_network_ids = [var.mainNetworkId]
}

resource "vcd_nsxt_distributed_firewall_rule" "outbound-any" {
  org          =  var.providerClusterConfiguration.organization
  vdc_group_id = data.vcd_vdc_group.test1.id

  name        = format("%s-outbound-any", var.providerClusterConfiguration.mainNetwork)
  action      = "ALLOW"
  description = format("Allow any outbound traffic from %s network", var.providerClusterConfiguration.mainNetwork)

  source_ids = [vcd_nsxt_security_group.network.id]
}
