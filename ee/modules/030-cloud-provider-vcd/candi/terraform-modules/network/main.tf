# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

locals {
  edge_gateway_id = local.use_nsxv ? data.vcd_edgegateway.gateway[0].id : data.vcd_nsxt_edgegateway.gateway[0].id
  use_nsxv        = var.edge_gateway_type == "NSX-V"
}

data "vcd_nsxt_edgegateway" "gateway" {
  count = local.use_nsxv ? 0 : 1
  org   = var.organization
  name  = var.edge_gateway_name
}

data "vcd_edgegateway" "gateway" {
  count = local.use_nsxv ? 1 : 0
  org   = var.organization
  name  = var.edge_gateway_name
}

resource "vcd_network_routed_v2" "network" {
  org  = var.organization
  name = var.internal_network_name

  edge_gateway_id = local.edge_gateway_id

  gateway       = cidrhost(var.internal_network_cidr, 1)
  prefix_length = tonumber(split("/", var.internal_network_cidr)[1])
  dns1          = length(var.internal_network_dns_servers) > 0 ? var.internal_network_dns_servers[0] : null
  dns2          = length(var.internal_network_dns_servers) > 1 ? var.internal_network_dns_servers[1] : null

  dynamic "metadata_entry" {
    for_each = var.metadata

    content {
      type        = "MetadataStringValue"
      is_system   = false
      user_access = "READWRITE"
      key         = metadata_entry.key
      value       = metadata_entry.value
    }
  }
}

resource "vcd_nsxt_network_dhcp" "pools" {
  count = local.use_nsxv ? 0 : 1

  org            = var.organization
  org_network_id = vcd_network_routed_v2.network.id
  dns_servers    = var.internal_network_dns_servers

  pool {
    start_address = cidrhost(var.internal_network_cidr, var.internal_network_dhcp_pool_start_address)
    end_address   = cidrhost(var.internal_network_cidr, -2)
  }

}
