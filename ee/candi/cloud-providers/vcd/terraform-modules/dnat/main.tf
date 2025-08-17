# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  use_nsxv = var.edge_gateway_type == "NSX-V"
}

data "vcd_nsxt_edgegateway" "gateway" {
  count = local.use_nsxv ? 0 : 1
  org   = var.organization
  name  = var.edge_gateway_name
}

# NSX-T DNAT rule

data "vcd_nsxt_app_port_profile" "ssh" {
  count = local.use_nsxv ? 0 : 1
  org   = var.organization
  name  = "SSH"
  scope = "SYSTEM"
}

resource "vcd_nsxt_nat_rule" "ssh_dnat" {
  count = local.use_nsxv ? 0 : 1
  org   = var.organization

  edge_gateway_id = data.vcd_nsxt_edgegateway.gateway[0].id

  name        = format("%s-dnat-ssh", var.rule_name_prefix)
  rule_type   = "DNAT"
  description = var.rule_description

  external_address    = var.external_address
  dnat_external_port  = var.external_port
  internal_address    = var.internal_address
  logging             = false
  app_port_profile_id = data.vcd_nsxt_app_port_profile.ssh[0].id
}

# NSX-V DNAT rule

resource "vcd_nsxv_dnat" "ssh_dnat" {
  count = local.use_nsxv ? 1 : 0

  enabled     = true
  description = var.rule_description
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
