# Copyright 2025 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE

# NSX-T resources

locals {
  use_nsxv = var.edge_gateway_type == "NSX-V"
}

data "vcd_nsxt_edgegateway" "gateway" {
  count = local.use_nsxv ? 0 : 1
  org   = var.organization
  name  = var.edge_gateway_name
}

resource "vcd_nsxt_ip_set" "internal_network" {
  count           = local.use_nsxv ? 0 : 1
  org             = var.organization
  edge_gateway_id = data.vcd_nsxt_edgegateway.gateway[0].id

  name         = var.internal_network_name
  description  = format("%s CIDR", var.internal_network_name)
  ip_addresses = [var.internal_network_cidr]
}

data "vcd_nsxt_app_port_profile" "ssh" {
  count = local.use_nsxv ? 0 : 1
  org   = var.organization

  name  = "SSH"
  scope = "SYSTEM"
}

data "vcd_nsxt_app_port_profile" "icmp" {
  count = local.use_nsxv ? 0 : 1
  org   = var.organization

  name  = "ICMP ALL"
  scope = "SYSTEM"
}

resource "vcd_nsxt_app_port_profile" "node_ports" {
  count       = local.use_nsxv ? 0 : 1
  org         = var.organization
  description = "Node ports for Kubernetes"

  name  = "NODE PORTS"
  scope = "TENANT"

  app_port {
    protocol = "TCP"
    port     = ["30000-32767"]
  }

  app_port {
    protocol = "UDP"
    port     = ["30000-32767"]
  }
}

resource "vcd_nsxt_firewall" "firewall" {
  count           = local.use_nsxv ? 0 : 1
  org             = var.organization
  edge_gateway_id = data.vcd_nsxt_edgegateway.gateway[0].id

  rule {
    enabled              = true
    action               = "ALLOW"
    name                 = format("%s-inbound-ssh", var.internal_network_name)
    direction            = "IN"
    ip_protocol          = "IPV4"
    source_ids           = []
    destination_ids      = [vcd_nsxt_ip_set.internal_network[0].id]
    app_port_profile_ids = [data.vcd_nsxt_app_port_profile.ssh[0].id]
  }

  rule {
    enabled              = true
    action               = "ALLOW"
    name                 = format("%s-inbound-icmp", var.internal_network_name)
    direction            = "IN"
    ip_protocol          = "IPV4"
    source_ids           = []
    destination_ids      = [vcd_nsxt_ip_set.internal_network[0].id]
    app_port_profile_ids = [data.vcd_nsxt_app_port_profile.icmp[0].id]
  }

  rule {
    enabled              = true
    action               = "ALLOW"
    name                 = format("%s-inbound-node-ports", var.internal_network_name)
    direction            = "IN"
    ip_protocol          = "IPV4"
    source_ids           = []
    destination_ids      = [vcd_nsxt_ip_set.internal_network[0].id]
    app_port_profile_ids = [vcd_nsxt_app_port_profile.node_ports[0].id]
  }

  rule {
    enabled         = true
    action          = "ALLOW"
    name            = format("%s-outbound-any", var.internal_network_name)
    direction       = "OUT"
    ip_protocol     = "IPV4"
    source_ids      = [vcd_nsxt_ip_set.internal_network[0].id]
    destination_ids = []
  }
}

# NSX-V resources

resource "vcd_nsxv_firewall_rule" "ssh" {
  count        = local.use_nsxv ? 1 : 0
  org          = var.organization
  edge_gateway = var.edge_gateway_name

  name = format("%s-inbound-ssh", var.internal_network_name)

  source {
    ip_addresses = ["any"]
  }

  destination {
    ip_addresses = [var.internal_network_cidr]
  }

  service {
    protocol = "tcp"
    port     = "22"
  }
}

resource "vcd_nsxv_firewall_rule" "icmp" {
  count        = local.use_nsxv ? 1 : 0
  org          = var.organization
  edge_gateway = var.edge_gateway_name

  name = format("%s-inbound-icmp", var.internal_network_name)

  source {
    ip_addresses = ["any"]
  }

  destination {
    ip_addresses = [var.internal_network_cidr]
  }

  service {
    protocol = "icmp"
  }
}

resource "vcd_nsxv_firewall_rule" "node_ports_tcp" {
  count        = local.use_nsxv ? 1 : 0
  org          = var.organization
  edge_gateway = var.edge_gateway_name

  name = format("%s-inbound-tcp-node-ports", var.internal_network_name)

  source {
    ip_addresses = ["any"]
  }

  destination {
    ip_addresses = [var.internal_network_cidr]
  }

  service {
    protocol = "tcp"
    port     = "30000-32767"
  }
}

resource "vcd_nsxv_firewall_rule" "node_ports_udp" {
  count        = local.use_nsxv ? 1 : 0
  org          = var.organization
  edge_gateway = var.edge_gateway_name

  name = format("%s-inbound-udp-node-ports", var.internal_network_name)

  source {
    ip_addresses = ["any"]
  }

  destination {
    ip_addresses = [var.internal_network_cidr]
  }

  service {
    protocol = "udp"
    port     = "30000-32767"
  }
}

resource "vcd_nsxv_firewall_rule" "outbound_any" {
  count        = local.use_nsxv ? 1 : 0
  org          = var.organization
  edge_gateway = var.edge_gateway_name

  name = format("%s-outbound-any", var.internal_network_name)

  source {
    ip_addresses = [var.internal_network_cidr]
  }

  destination {
    ip_addresses = ["any"]
  }

  service {
    protocol = "any"
  }
}
