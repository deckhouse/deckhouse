# Copyright 2023 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

output "name" {
  value = vcd_network_routed_v2.network.name  
}

output "networkId" {
  value = vcd_network_routed_v2.network.id
}

output "edgeGatewayId" {
  value = local.edgeGatewayId
}
