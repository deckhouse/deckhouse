# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  is_first_node = var.node_index == 0
  use_nsxv      = var.edge_gateway_type == "NSX-V"
}

data "vcd_nsxt_edgegateway" "gateway" {
  count = local.use_nsxv ? 0 : 1
  org   = var.organization
  name  = var.edge_gateway_name
}

# NSX-T DNAT rule only for the first master node

data "vcd_nsxt_app_port_profile" "ssh" {
  count = local.is_first_node ? (local.use_nsxv ? 0 : 1) : 0
  org   = var.organization
  name  = "SSH"
  scope = "SYSTEM"
}

resource "vcd_nsxt_nat_rule" "master-dnat" {
  count = local.is_first_node && local.use_nsxv ? 0 : 1
  org   = var.organization

  edge_gateway_id = data.vcd_nsxt_edgegateway.gateway[0].id

  name        = format("%s-dnat-ssh", var.internal_network_name)
  rule_type   = "DNAT"
  description = format("SSH DNAT rule for first master of %s", var.internal_network_name)

  external_address    = var.external_address
  dnat_external_port  = var.external_port
  internal_address    = var.internal_address
  logging             = false
  app_port_profile_id = data.vcd_nsxt_app_port_profile.ssh[0].id
}

# NSX-V DNAT rule only for the first master node

resource "vcd_nsxv_dnat" "master-dnat" {
  count = local.is_first_node && local.use_nsxv ? 1 : 0

  enabled     = true
  description = format("SSH DNAT rule for first master of %s", var.internal_network_name)
  org         = var.organization

  edge_gateway = var.edge_gateway_name
  network_type = var.external_network_type
  network_name = var.external_network_name

  original_address   = var.external_address
  original_port      = var.external_port
  translated_address = var.internal_address
  protocol           = "tcp"
  translated_port    = 22
}

