# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  use_nsxv = var.edge_gateway_type == "NSX-V"
}

# NSX-T resources

data "vcd_nsxt_edgegateway" "gateway" {
  count = local.use_nsxv ? 0 : 1
  org   = var.organization
  name  = var.edge_gateway_name
}

resource "vcd_nsxt_nat_rule" "snat" {
  count = local.use_nsxv ? 0 : 1
  org   = var.organization

  edge_gateway_id = data.vcd_nsxt_edgegateway.gateway[0].id

  name        = format("%s-snat", var.rule_name_prefix)
  rule_type   = "SNAT"
  description = var.rule_description

  external_address = var.external_address
  internal_address = var.internal_network_cidr
  logging          = false
}

# NSX-V resources

resource "vcd_nsxv_snat" "snat" {
  count = local.use_nsxv ? 1 : 0

  enabled     = true
  description = format("SNAT rule for %s", var.internal_network_name)
  org         = var.organization

  edge_gateway = var.edge_gateway_name
  network_type = var.external_network_type
  network_name = var.external_network_name

  original_address   = var.internal_network_cidr
  translated_address = var.external_address
}
