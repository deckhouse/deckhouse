# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "name" {
  value = vcd_vapp_network_routed.network.name
}

output "networkId" {
  value = vcd_vapp_network_routed.network.id
}

output "edgeGatewayId" {
  value = local.edgeGatewayId
}
